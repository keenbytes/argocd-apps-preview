package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"argocd-app-of-apps-diff-preview/internal/argocd"
	"argocd-app-of-apps-diff-preview/internal/command"
	"argocd-app-of-apps-diff-preview/internal/kind"
	"argocd-app-of-apps-diff-preview/internal/kube"
	"argocd-app-of-apps-diff-preview/internal/logmsg"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	maxRecursions                  = 7
	ctxClusterTimeoutSeconds       = 120
	ctxArgoCDTimeoutSeconds        = 360
	sleepSecondsAfterArgoCDInstall = 4
)

const (
	kindName        = "argocd-app-prev"
	kindImage       = "kindest/node:v1.33.4"
	argoCDNamespace = "tools"
	argoCDVersion   = "v2.14.11"
	argoCDNodePort  = 30443
)

const (
	exitKindNotFound                  = 101
	exitArgoCDNotFound                = 102
	exitKubectlNotFound               = 103
	exitDirNotFound                   = 104
	exitOutputsDirNotFound            = 105
	exitOutputsDirNotEmpty            = 106
	exitCreatingClusterFailed         = 201
	exitArgoCDInstallationFailed      = 301
	exitArgoCDLoggingFailed           = 303
	exitApplyingSecretsFailed         = 304
	exitApplyingManifestsFailed       = 305
	exitRecursivelyApplyingAppsFailed = 306
	exitDumpingAppManifestsFailed     = 307
)

var (
	regexpRepoURL = regexp.MustCompile(`^(https:\/\/|ssh:\/\/|git@)[\w\-\.]+(:|\/)[\w\-\/]+(\.git)?$`)
	regexpGitRef  = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

func main() {
	var outputAppsDir string
	var hooksDir string
	var manifestsDir string
	var secretsDir string
	var repoURL string
	var targetRevision string

	rootCmd := &cobra.Command{
		Use:   "apps",
		Short: "ArgoCD Apps Preview",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if repoURL != "" || targetRevision != "" {
				if !regexpRepoURL.MatchString(repoURL) {
					return fmt.Errorf("invalid repository URL: %s", repoURL)
				}
				if !regexpGitRef.MatchString(targetRevision) {
					return fmt.Errorf("invalid target revision: %s", targetRevision)
				}
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			checkPrerequisites()
			checkDirs(manifestsDir, secretsDir, hooksDir, outputAppsDir)
			execute(manifestsDir, secretsDir, hooksDir, outputAppsDir, repoURL, targetRevision)
		},
	}

	rootCmd.Flags().StringVar(&manifestsDir, "manifests", "", "Directory with start manifests")
	rootCmd.Flags().StringVar(&secretsDir, "secrets", "", "Directory with secrets")
	rootCmd.Flags().StringVar(&hooksDir, "hooks", "", "Directory for hooks scripts")
	rootCmd.Flags().StringVar(&outputAppsDir, "output-apps", "", "Directory to output app manifests")
	rootCmd.Flags().StringVar(&repoURL, "replace-repo-url", "", "Repository URL to replace")
	rootCmd.Flags().StringVar(&targetRevision, "replace-target-revision", "", "Target revision to replace")
	_ = rootCmd.MarkFlagRequired("manifests")
	_ = rootCmd.MarkFlagRequired("outputs")

	err := rootCmd.Execute()
	if err != nil {
		logmsg.Error("Error executing command", err)
		os.Exit(1)
	}
}

func execute(manifestsDir, secretsDir, hooksDir, outputAppsDir, repoURL, revision string) {
	if repoURL != "" && revision != "" {
		logmsg.Info(fmt.Sprintf("Changing repository URL and target revision to: %s %s", repoURL, revision))
	}

	// start kind cluster
	cluster := kind.NewKind(kindName, kindImage)
	_ = cluster.Delete()

	ctxCluster, cancelCluster := context.WithTimeout(context.Background(), ctxClusterTimeoutSeconds*time.Second)
	defer cancelCluster()

	err := cluster.Create(ctxCluster)
	if err != nil {
		logmsg.Error("creating kind cluster failed", err)
		_ = cluster.Delete()
		os.Exit(exitCreatingClusterFailed)
	}
	defer cluster.Delete()

	// get kube client
	kubeClient := kube.NewKube(getKubeContext())

	// install argocd on the kind cluster
	acd := argocd.NewArgoCD(kubeClient, argoCDNamespace, argoCDVersion, argoCDNodePort)

	// add timeout to context for argocd installation
	ctxArgoCD, cancelArgoCD := context.WithTimeout(context.Background(), ctxArgoCDTimeoutSeconds*time.Second)
	defer cancelArgoCD()

	err = acd.Install(ctxArgoCD)
	if err != nil {
		logmsg.Error("argocd installation failed", err)
		os.Exit(exitArgoCDInstallationFailed)
	}

	// wait a while until argocd starts up
	time.Sleep(sleepSecondsAfterArgoCDInstall * time.Second)

	// log in to argocd
	err = acd.Login(ctxArgoCD)
	if err != nil {
		logmsg.Error("logging in to argocd failed", err)
		os.Exit(exitArgoCDLoggingFailed)
	}

	// apply manifests from the secrets (to allow argocd pull from private repositories etc.)
	if secretsDir != "" {
		err = applyManifests(ctxArgoCD, kubeClient, secretsDir, argoCDNamespace)
		if err != nil {
			logmsg.Error("applying secrets failed", err)
			os.Exit(exitApplyingSecretsFailed)
		}
	}

	// apply initial manifests (we need to start somewhere)
	err = applyAppManifestsFromDir(ctxArgoCD, kubeClient, acd, manifestsDir, hooksDir, [2]string{repoURL, revision})
	if err != nil {
		logmsg.Error("applying initial manifests failed", err)
		os.Exit(exitApplyingManifestsFailed)
	}

	// add timeout to context for getting the applications recursively
	ctxRecursiveApply, cancelRecursiveApply := context.WithTimeout(context.Background(), 360*time.Second)
	defer cancelRecursiveApply()

	// process argocd applications recursively
	logmsg.Info("Starting to recursively apply applications...")
	numRecursions := 0
	processedApps := map[string]struct{}{}
	err = recursivelyApplyApps(ctxRecursiveApply, kubeClient, acd, &numRecursions, &processedApps, hooksDir, [2]string{repoURL, revision})
	if err != nil {
		logmsg.Error("recursive applying apps failed", err)
		os.Exit(exitRecursivelyApplyingAppsFailed)
	}
	logmsg.Info("Finished recursively applying applications.")

	// dump app manifests to the outputs directory
	err = dumpAppManifests(ctxArgoCD, acd, outputAppsDir)
	if err != nil {
		logmsg.Error("dumping app manifests failed", err)
		os.Exit(exitDumpingAppManifestsFailed)
	}
	logmsg.Info(fmt.Sprintf("Finished dumping app manifests to %s directory.", outputAppsDir))
}

func getKubeContext() string {
	return fmt.Sprintf("kind-%s", kindName)
}

func executeHookIfExists(ctx context.Context, hookPath string, envVars map[string]string) error {
	info, err := os.Stat(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("getting stat for hook file %s: %w", hookPath, err)
	}
	if info.IsDir() {
		return nil
	}

	logmsg.Info(fmt.Sprintf("Executing hook: %s", hookPath))
	cmd, err := command.NewCommand("bash", hookPath)
	if err != nil {
		return fmt.Errorf("creating command for hook %s: %w", hookPath, err)
	}
	err = cmd.Run(ctx, &envVars)
	if err != nil {
		return fmt.Errorf("executing hook %s: %w", hookPath, err)
	}
	return nil
}

func applyManifests(ctx context.Context, kubeClient *kube.Kube, dir string, namespace string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("getting stat for directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", dir)
	}

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == ".yaml" || ext == ".yml" {
			err2 := kubeClient.ApplyFile(ctx, path, namespace)
			if err2 != nil {
				return fmt.Errorf("applying manifest from %s: %w", dir, err2)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking directory %s: %v", dir, err)
	}

	return nil
}

func applyAppManifestsFromDir(ctx context.Context, kubeClient *kube.Kube, acd *argocd.ArgoCD, dir string, hooksDir string, target [2]string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("getting stat for directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", dir)
	}

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == ".yaml" || ext == ".yml" {
			_, err2 := extractAndApplyAppsFromManifestsYAML(ctx, path, kubeClient, acd, hooksDir, target)
			if err2 != nil {
				return fmt.Errorf("extracting and applying apps from manifests yaml: %w", err2)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking directory %s: %v", dir, err)
	}

	return nil
}

func extractAndApplyAppsFromManifestsYAML(ctx context.Context, path string, kubeClient *kube.Kube, acd *argocd.ArgoCD, hooksDir string, target [2]string) (bool, error) {
	apps, appSets, appProjects, err := kube.ExtractAppsFromYAML(path)
	if err != nil {
		return false, fmt.Errorf("extracting apps from yaml: %w", err)
	}

	if len(appProjects) > 0 {
		for _, appProject := range appProjects {
			err2 := kubeClient.ApplyFile(ctx, appProject, acd.Namespace())
			if err2 != nil {
				return false, fmt.Errorf("applying app project manifest from %s: %w", appProject, err2)
			}
		}
	}

	if len(appSets) > 0 {
		for _, appSet := range appSets {
			hookPath := filepath.Join(hooksDir, "before-appset-gen.sh")
			err := executeHookIfExists(ctx, hookPath, map[string]string{
				"APPSET_YAML": appSet,
			})
			if err != nil {
				return false, fmt.Errorf("executing before-appset-gen hook: %w", err)
			}

			if target[0] != "" && target[1] != "" {
				modifiedAppSet, err := replaceRepoURLAndTargetRevision(appSet, target[0], target[1])
				if err != nil {
					return false, fmt.Errorf("replacing repo URL and target revision in appset %s: %w", appSet, err)
				}

				appSet = modifiedAppSet
			}

			genApps, err := acd.GenerateAppsFromAppSets(ctx, appSet)
			if err != nil {
				base := filepath.Base(appSet)

				return false, fmt.Errorf("generating apps from appset %s: %w", base, err)
			}

			for _, genApp := range genApps {
				apps = append(apps, genApp)
			}
		}
	}

	if len(apps) == 0 {
		return false, nil
	}

	added := false
	for _, app := range apps {
		hookPath := filepath.Join(hooksDir, "before-app-apply.sh")
		err := executeHookIfExists(ctx, hookPath, map[string]string{
			"APP_YAML": app,
		})
		if err != nil {
			return false, fmt.Errorf("executing before-app-apply hook: %w", err)
		}

		if target[0] != "" && target[1] != "" {
			modifiedApp, err := replaceRepoURLAndTargetRevision(app, target[0], target[1])
			if err != nil {
				return false, fmt.Errorf("replacing repo URL and target revision in appset %s: %w", app, err)
			}

			app = modifiedApp
		}

		err2 := kubeClient.ApplyFile(ctx, app, acd.Namespace())
		if err2 != nil {
			return added, fmt.Errorf("applying app manifest from %s: %w", app, err2)
		}

		added = true
	}

	return added, nil
}

func recursivelyApplyApps(ctx context.Context, kubeClient *kube.Kube, acd *argocd.ArgoCD, numRecursions *int, processedApps *map[string]struct{}, hooksDir string, target [2]string) error {
	*numRecursions++
	if *numRecursions > maxRecursions {
		return fmt.Errorf("max recursions of %d reached", maxRecursions)
	}

	added := false
	appList, err := acd.GetAppList(ctx)
	if err != nil {
		return fmt.Errorf("getting app list using argocd: %w", err)
	}

	for _, appListItem := range appList {
		app := strings.TrimSpace(appListItem)
		if app == "" {
			continue
		}

		logmsg.Info(fmt.Sprintf("Scanning application %s (recursion: %d)...", app, *numRecursions))

		// check if the app has been already processed
		_, ok := (*processedApps)[app]
		if ok {
			continue
		}

		err := acd.WaitForAppManifests(ctx, app)
		if err != nil {
			return fmt.Errorf("waiting for app %s manifests: %w", app, err)
		}

		manifests, err := acd.GetAppManifests(ctx, app)
		if err != nil {
			return fmt.Errorf("getting app %s manifests: %w", app, err)
		}

		addedApps, err := extractAndApplyAppsFromManifestsYAML(ctx, manifests, kubeClient, acd, hooksDir, target)
		if err != nil {
			return fmt.Errorf("extracting and applying apps from manifests yaml: %w", err)
		}

		if addedApps {
			added = true
		}

		(*processedApps)[app] = struct{}{}
	}

	if added {
		err := recursivelyApplyApps(ctx, kubeClient, acd, numRecursions, processedApps, hooksDir, target)
		if err != nil {
			return fmt.Errorf("recursively applying apps (recursion: %d): %w", *numRecursions, err)
		}
	}

	return nil
}

func replaceRepoURLAndTargetRevision(appSet string, repoURL string, targetRevision string) (string, error) {
	logmsg.Info(fmt.Sprintf("Changing target revision to %s in repository %s...", targetRevision, repoURL))

	appSetYAML, err := os.ReadFile(appSet)
	if err != nil {
		return "", fmt.Errorf("reading appset %s: %w", appSet, err)
	}

	var rootNode yaml.Node
	err = yaml.Unmarshal(appSetYAML, &rootNode)
	if err != nil {
		return "", fmt.Errorf("parsing YAML document: %w", err)
	}

	updated := false
	var traverse func(node *yaml.Node) error
	traverse = func(node *yaml.Node) error {
		if node == nil {
			return nil
		}

		if node.Kind != yaml.MappingNode {
			for _, child := range node.Content {
				err := traverse(child)
				if err != nil {
					return err
				}
			}

			return nil
		}

		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == "repoURL" && value.Value == repoURL {
				logmsg.Info(fmt.Sprintf("Found key %s with value %s...", key.Value, value.Value))
				for j := 0; j < len(node.Content); j += 2 {
					siblingKey := node.Content[j]
					siblingValue := node.Content[j+1]

					if siblingKey.Value == "revision" || siblingKey.Value == "targetRevision" {
						logmsg.Info(fmt.Sprintf("Found sibling key %s with value %s...", siblingKey.Value, siblingValue.Value))
						siblingValue.Value = targetRevision
						updated = true
					}
				}
			}

			err := traverse(value)
			if err != nil {
				return err
			}
		}

		return nil
	}

	err = traverse(&rootNode)
	if err != nil {
		return "", fmt.Errorf("traversing YAML document: %w", err)
	}

	if !updated {
		return appSet, nil
	}

	logmsg.Info(fmt.Sprintf("Changed targetRevision in %s...", appSet))
	newFileName := filepath.Join(filepath.Dir(appSet), "B_"+filepath.Base(appSet))
	newYAML, err := yaml.Marshal(&rootNode)
	if err != nil {
		return "", fmt.Errorf("encoding updated YAML: %w", err)
	}

	err = os.WriteFile(newFileName, newYAML, 0644)
	if err != nil {
		return "", fmt.Errorf("writing updated YAML to file %s: %w", newFileName, err)
	}

	return newFileName, nil

}

func dumpAppManifests(ctx context.Context, acd *argocd.ArgoCD, dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory %s does not exist", dir)
		}
		return fmt.Errorf("getting stat for directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading output directory %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("output directory %s is not empty", dir)
	}

	appList, err := acd.GetAppList(ctx)
	if err != nil {
		return fmt.Errorf("getting app list using argocd: %w", err)
	}

	for _, appListItem := range appList {
		app := strings.TrimSpace(appListItem)
		if app == "" {
			continue
		}

		err := acd.WaitForAppManifests(ctx, app)
		if err != nil {
			return fmt.Errorf("waiting for app %s manifests: %w", app, err)
		}

		manifestsFile, err := acd.GetAppManifests(ctx, app)
		if err != nil {
			return fmt.Errorf("getting app %s manifests: %w", app, err)
		}

		outputFilename := strings.ReplaceAll(app, "/", "__")

		// copy manifestsFile to dir/outputFilename.yaml
		srcFile, err := os.Open(manifestsFile)
		if err != nil {
			return fmt.Errorf("opening app manifests file %s: %w", manifestsFile, err)
		}
		defer func() {
			err := srcFile.Close()
			if err != nil {
				logmsg.Error("error closing app manifests file "+manifestsFile, err)
			}
		}()

		dstPath := filepath.Join(dir, outputFilename+".yaml")
		dstFile, err := os.Create(dstPath)
		if err != nil {
			err2 := srcFile.Close()
			if err2 != nil {
				return fmt.Errorf("closing manifests file %s: %w", dstPath, err2)
			}
			return fmt.Errorf("creating dump manifests file %s: %w", dstPath, err)
		}
		defer func() {
			err := dstFile.Close()
			if err != nil {
				logmsg.Error("error closing dump manifests file "+manifestsFile, err)
			}
		}()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return fmt.Errorf("copying file from %s to %s: %w", manifestsFile, dstPath, err)
		}

		err = dstFile.Sync()
		if err != nil {
			return fmt.Errorf("writing app %s manifests to file: %w", app, err)
		}
	}
	return nil
}

func checkPrerequisites() {
	logmsg.Info("Checking prerequisites...")

	_, err := exec.LookPath("kind")
	if err != nil {
		logmsg.Error("kind not found in PATH", nil)
		os.Exit(exitKindNotFound)
	}

	_, err = exec.LookPath("argocd")
	if err != nil {
		logmsg.Error("argocd not found in PATH", nil)
		os.Exit(exitArgoCDNotFound)
	}

	_, err = exec.LookPath("kubectl")
	if err != nil {
		logmsg.Error("kubectl not found in PATH", nil)
		os.Exit(exitKubectlNotFound)
	}
}

func checkDirs(manifests, secrets, hooks, outputs string) {
	for _, dir := range [4]string{manifests, secrets, hooks, outputs} {
		if dir == "" {
			continue
		}

		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				logmsg.Error(fmt.Sprintf("%s directory not found: %v", dir, err), nil)
				os.Exit(exitDirNotFound)
			}
			logmsg.Error(fmt.Sprintf("Error checking %s directory: %v", dir, err), nil)
			os.Exit(exitDirNotFound)
		}
		if !info.IsDir() {
			logmsg.Error(fmt.Sprintf("%s exists but is not a directory", dir), nil)
			os.Exit(exitDirNotFound)
		}
	}
	entries, err := os.ReadDir(outputs)
	if err != nil {
		logmsg.Error(fmt.Sprintf("Error reading %s directory: %v", outputs, err), nil)
		os.Exit(exitOutputsDirNotFound)
	}
	if len(entries) > 0 {
		logmsg.Error(fmt.Sprintf("output dir %s not empty: %v", outputs, err), nil)
		os.Exit(exitOutputsDirNotEmpty)
	}
}
