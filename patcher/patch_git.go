package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var injectStr = `
	// --- GIT STATUS CHECK ---
	{
		overrideGit := false
		for _, arg := range os.Args {
			if arg == "-dirty" {
				overrideGit = true
			}
		}
		out, _ := exec.Command("git", "rev-parse", "HEAD").Output()
		hash := strings.TrimSpace(string(out))
		fmt.Printf("HEAD: %s\n", hash)
		statOut, _ := exec.Command("git", "status", "--short").Output()
		stat := strings.TrimSpace(string(statOut))
		if stat != "" {
			fmt.Printf("Tree is dirty:\n%s\n", stat)
			if !overrideGit {
				fmt.Println("Exiting. Use -dirty to override.")
				os.Exit(1)
			}
		}
	}
	// ------------------------
`

func patchFile(path string) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	content := string(b)
	if strings.Contains(content, "git status") {
		return
	}

	// Add os/exec import safely
	if !strings.Contains(content, "\"os/exec\"") {
		// Find the import block
		idx := strings.Index(content, "import (")
		if idx != -1 {
			insert := "\n\t\"os/exec\""
			if !strings.Contains(content, "\"os\"") {
				insert += "\n\t\"os\""
			}
			if !strings.Contains(content, "\"strings\"") {
				insert += "\n\t\"strings\""
			}
			if !strings.Contains(content, "\"fmt\"") {
				insert += "\n\t\"fmt\""
			}
			content = content[:idx+8] + insert + content[idx+8:]
		} else {
			// Find package main
			idx = strings.Index(content, "package main")
			if idx != -1 {
				insert := "\nimport (\n\t\"os/exec\"\n\t\"os\"\n\t\"strings\"\n\t\"fmt\"\n)\n"
				content = content[:idx+12] + insert + content[idx+12:]
			}
		}
	}

	content = strings.Replace(content, "func main() {", "func main() {"+injectStr, 1)
	ioutil.WriteFile(path, []byte(content), 0644)
	fmt.Println("Patched", path)
}

func main() {
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, "main.go") && !strings.Contains(path, "vendor") && !strings.Contains(path, "patcher") {
			patchFile(path)
		}
		return nil
	})
}
