package main

import (
	"argocd-apps-preview/internal/diff"
	"argocd-apps-preview/internal/logmsg"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	exitDirNotFound        = 104
	exitOutputsDirNotFound = 105
	exitOutputsDirNotEmpty = 106
	exitGitNotFound        = 107
	exitBashNotFound       = 108
	exitGitInit            = 401
)

const (
	dirSplit = "split"
)

func main() {
	var appsBaseDir string
	var appsTargetDir string
	var outputDiffDir string

	rootCmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff apps",
		Run: func(cmd *cobra.Command, args []string) {
			checkPrerequisites()
			checkDirs(appsBaseDir, appsTargetDir, outputDiffDir)
			execute(appsBaseDir, appsTargetDir, outputDiffDir)
		},
	}

	rootCmd.Flags().StringVar(&appsBaseDir, "apps-base", "", "Manifests from base revision")
	rootCmd.Flags().StringVar(&appsTargetDir, "apps-target", "", "Manifests from target revision")
	rootCmd.Flags().StringVar(&outputDiffDir, "output-diff", "", "Directory to output diff")
	_ = rootCmd.MarkFlagRequired("apps-base")
	_ = rootCmd.MarkFlagRequired("apps-target")
	_ = rootCmd.MarkFlagRequired("output-diff")

	err := rootCmd.Execute()
	if err != nil {
		logmsg.Error("Error executing command", err)
		os.Exit(1)
	}
}

func execute(appsBaseDir, appsTargetDir, outputDiffDir string) {
	diffFile, err := diff.GenerateGitDiff(appsBaseDir, appsTargetDir, outputDiffDir)
	if err != nil {
		logmsg.Error("git init failed", err)
		os.Exit(exitGitInit)
	}
	logmsg.Info("Diff generated successfully in " + diffFile + ".")

	splitDir := filepath.Join(outputDiffDir, dirSplit)
	err = os.Mkdir(splitDir, os.ModePerm)
	if err != nil {
		logmsg.Error("failed to create '"+dirSplit+"' directory in "+outputDiffDir, err)
		os.Exit(exitGitInit)
	}

	err = diff.SplitGitDiff(diffFile, splitDir)
	if err != nil {
		logmsg.Error("failed to split diff", err)
	}

	err = diff.AddResourceAbove(splitDir, appsBaseDir, appsTargetDir)
	if err != nil {
		logmsg.Error("failed to process split dir", err)
	}
}

func checkPrerequisites() {
	logmsg.Info("Checking prerequisites...")

	_, err := exec.LookPath("git")
	if err != nil {
		logmsg.Error("git not found in PATH", nil)
		os.Exit(exitGitNotFound)
	}

	_, err = exec.LookPath("bash")
	if err != nil {
		logmsg.Error("bash not found in PATH", nil)
		os.Exit(exitBashNotFound)
	}
}

func checkDirs(appsBase, appsTarget, outputDiff string) {
	for _, dir := range [4]string{appsBase, appsTarget, outputDiff} {
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
	entries, err := os.ReadDir(outputDiff)
	if err != nil {
		logmsg.Error(fmt.Sprintf("Error reading %s directory: %v", outputDiff, err), nil)
		os.Exit(exitOutputsDirNotFound)
	}
	if len(entries) > 0 {
		logmsg.Error(fmt.Sprintf("output diff directory %s not empty", outputDiff), nil)
		os.Exit(exitOutputsDirNotEmpty)
	}
}
