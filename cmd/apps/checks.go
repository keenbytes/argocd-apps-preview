package main

import (
	"fmt"
	"os"
	"os/exec"

	"argocd-apps-preview/internal/logmsg"
)

func checkPrerequisites() {
	logmsg.Info("Checking prerequisites...")

	_, err := exec.LookPath("kind")
	if err != nil {
		logmsg.Error(ErrMsgKindNotFound, nil)
		os.Exit(ExitKindNotFound)
	}

	_, err = exec.LookPath("argocd")
	if err != nil {
		logmsg.Error(ErrMsgArgoCDNotFound, nil)
		os.Exit(ExitArgoCDNotFound)
	}

	_, err = exec.LookPath("kubectl")
	if err != nil {
		logmsg.Error(ErrMsgKubectlNotFound, nil)
		os.Exit(ExitKubectlNotFound)
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
				os.Exit(ExitDirNotFound)
			}
			logmsg.Error(fmt.Sprintf("Error checking %s directory: %v", dir, err), nil)
			os.Exit(ExitDirNotFound)
		}
		if !info.IsDir() {
			logmsg.Error(fmt.Sprintf("%s exists but is not a directory", dir), nil)
			os.Exit(ExitDirNotFound)
		}
	}
	entries, err := os.ReadDir(outputs)
	if err != nil {
		logmsg.Error(fmt.Sprintf("Error reading %s directory: %v", outputs, err), nil)
		os.Exit(ExitOutputsDirNotFound)
	}
	if len(entries) > 0 {
		logmsg.Error(ErrMsgOutputsDirNotEmpty, nil)
		os.Exit(ExitOutputsDirNotEmpty)
	}
}
