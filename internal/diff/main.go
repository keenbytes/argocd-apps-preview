package diff

import (
	"argocd-app-of-apps-diff-preview/internal/command"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	gitDiffFilename = "git-diff.txt"
)

var (
	regexpDiffHeader = regexp.MustCompile(`^diff --git a/`)
	regexpHunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
	regexpDiffAFile  = regexp.MustCompile(`^--- a/(.+)$`)
	regexpDiffBFile  = regexp.MustCompile(`^\+\+\+ b/(.+)$`)
)

type hunkInfo struct {
	lineNumber int // 1-indexed line number in the diff file
	posA       int // starting line in the "a" (source) version
	posB       int // starting line in the "b" (target) version
}

type resourceInfo struct {
	Kind string
	Name string
}

func GenerateGitDiff(appsBaseDir, appsTargetDir, outputDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "argocd-app-of-apps-diff-preview-git-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	appsBaseFiles := filepath.Join(appsBaseDir, "*.yaml")
	appsTargetFiles := filepath.Join(appsTargetDir, "*.yaml")
	outputFile := filepath.Join(outputDir, gitDiffFilename)

	// bash is used to expand the globs
	commandsToRun := [][]string{
		{"git", "init", tmpDir, "--quiet"},
		{"bash", "-c", "cp -rf " + appsBaseFiles + " " + tmpDir + "/"},
		{"git", "-C", tmpDir, "config", "user.email", "email@example.com"},
		{"git", "-C", tmpDir, "config", "user.name", "Name"},
		{"git", "-C", tmpDir, "add", "."},
		{"git", "-C", tmpDir, "commit", "-m", "base"},
		{"bash", "-c", "rm " + tmpDir + "/*.yaml"},
		{"bash", "-c", "cp -rf " + appsTargetFiles + " " + tmpDir + "/"},
		{"git", "-C", tmpDir, "add", "."},
		{"git", "-C", tmpDir, "commit", "-m", "base", "--allow-empty"},
		{"bash", "-c", "git -C " + tmpDir + " --no-pager diff HEAD~1 HEAD > " + outputFile},
	}

	for _, commandArgs := range commandsToRun {
		commandInstance, err := command.NewCommand(commandArgs[0], commandArgs[1:]...)
		if err != nil {
			return "", fmt.Errorf("creating gen git diff command failed: %w", err)
		}

		err = commandInstance.Run(context.Background(), nil)
		if err != nil {
			return "", fmt.Errorf("running gen git diff failed: %w", err)
		}
	}

	return outputFile, nil
}

func SplitGitDiff(gitDiffFile, diffSplitDir string) error {
	file, err := os.Open(gitDiffFile)
	if err != nil {
		return fmt.Errorf("opening file %s failed: %w", gitDiffFile, err)
	}
	defer file.Close()

	var outputFile *os.File
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if regexpDiffHeader.MatchString(line) {
			// Close previous file if exists
			if outputFile != nil {
				outputFile.Close()
			}

			// Extract filename from "diff --git a/path/to/file b/path/to/file"
			// We want the part after "b/"
			parts := strings.SplitN(line, "b/", 2)
			if len(parts) < 2 {
				fmt.Fprintf(os.Stderr, "Warning: Could not parse filename from line: %s\n", line)
				continue
			}
			filename := parts[1]

			// Create output file path
			outputPath := filepath.Join(diffSplitDir, filename+".diff")

			// Create/open the output file
			outputFile, err = os.Create(outputPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating file %s: %v\n", outputPath, err)
				continue
			}

			fmt.Fprintf(os.Stderr, "Writing to: %s\n", outputPath)
		}

		// Write line to current output file if one is open
		if outputFile != nil {
			if _, err := fmt.Fprintln(outputFile, line); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
				outputFile.Close()
				outputFile = nil
			}
		}
	}

	// Close the last open file
	if outputFile != nil {
		outputFile.Close()
	}
	return nil
}

func AddResourceAbove(diffSplitDir, appsBaseDir, appsTargetDir string) error {
	pattern := filepath.Join(diffSplitDir, "*.diff")
	diffFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("Error globbing diff files: %v\n", err)
	}

	for _, diffFile := range diffFiles {
		err = addResourceAboveFile(diffFile, appsBaseDir, appsTargetDir)
		if err != nil {
			return fmt.Errorf("Error processing diff file %s: %v\n", diffFile, err)
		}
	}

	return nil
}

func addResourceAboveFile(diffFilePath, appsBaseDir, appsTargetDir string) error {
	lines, err := readLines(diffFilePath)
	if err != nil {
		return fmt.Errorf("Error reading %s: %v\n", diffFilePath, err)
	}

	filenameA := ""
	for _, line := range lines {
		if m := regexpDiffAFile.FindStringSubmatch(line); m != nil {
			filenameA = m[1]
			break
		}
	}

	filenameB := ""
	for _, line := range lines {
		if m := regexpDiffBFile.FindStringSubmatch(line); m != nil {
			filenameB = m[1]
			break
		}
	}

	if filenameA == "/dev/null" {
		filenameA = ""
	}
	if filenameB == "/dev/null" {
		filenameB = ""
	}

	manifestPathA := filepath.Join(appsBaseDir, filenameA)
	manifestPathB := filepath.Join(appsTargetDir, filenameB)

	var hunks []hunkInfo
	for i, line := range lines {
		if m := regexpHunkHeader.FindStringSubmatch(line); m != nil {
			posA, _ := strconv.Atoi(m[1])
			posB, _ := strconv.Atoi(m[2])
			hunks = append(hunks, hunkInfo{
				lineNumber: i + 1, // 1-indexed
				posA:       posA,
				posB:       posB,
			})
		}
	}

	// Process hunks in order, inserting comments and tracking the offset
	// caused by previously inserted lines
	numAddedLines := 0

	for _, hunk := range hunks {
		adjustedLineNumber := hunk.lineNumber + numAddedLines

		if filenameA != "" && fileExists(manifestPathA) {
			info, found := lookupResource(manifestPathA, hunk.posA)
			if !found {
				// Both kind and name were null — skip the entire hunk
				continue
			}

			comment := fmt.Sprintf("## a/resource above: %+v", info)
			insertIdx := adjustedLineNumber - 1 // convert to 0-indexed
			lines = insertLine(lines, insertIdx, comment)
			numAddedLines++
			adjustedLineNumber++
		}

		if filenameB != "" && fileExists(manifestPathB) {
			info, found := lookupResource(manifestPathB, hunk.posB)
			if !found {
				continue
			}

			comment := fmt.Sprintf("## b/resource above: %+v", info)
			insertIdx := adjustedLineNumber - 1
			lines = insertLine(lines, insertIdx, comment)
			numAddedLines++
		}
	}

	// Write the modified file back
	if err := writeLines(diffFilePath, lines); err != nil {
		return fmt.Errorf("Error writing %s: %v\n", diffFilePath, err)
	}

	return nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func lookupResource(manifestPath string, pos int) (*resourceInfo, bool) {
	allLines, err := readLines(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading manifest %s: %v\n", manifestPath, err)
		return nil, false
	}

	if pos > len(allLines) {
		pos = len(allLines)
	}
	if pos <= 0 {
		return nil, false
	}

	subset := strings.Join(allLines[:pos], "\n")

	var lastKind, lastName string
	decoder := yaml.NewDecoder(strings.NewReader(subset))

	for {
		var doc map[string]interface{}
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			// Skip malformed documents
			continue
		}

		if kind, ok := doc["kind"].(string); ok && kind != "" {
			lastKind = kind
		}
		if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok && name != "" {
				lastName = name
			}
		}
	}

	// Both null → skip (equivalent to: [[ "${kind}" == "null" && "${name}" == "null" ]] && continue)
	if lastKind == "" && lastName == "" {
		return nil, false
	}

	if lastKind == "" {
		lastKind = "(unknown)"
	}
	if lastName == "" {
		lastName = "(unknown)"
	}

	return &resourceInfo{Kind: lastKind, Name: lastName}, true
}

func insertLine(lines []string, index int, line string) []string {
	lines = append(lines, "")
	copy(lines[index+1:], lines[index:])
	lines[index] = line
	return lines
}

func writeLines(path string, lines []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(writer, line)
	}
	return writer.Flush()
}
