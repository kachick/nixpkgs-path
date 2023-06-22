package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	re = regexp.MustCompile(`(?s)(import\s+\(fetchTarball\s+"https://github.com/NixOS/nixpkgs/archive/)([^"]+?)(\.tar\.gz"\))`)
)

func main() {
	usageError := errors.New("expected 'bump' or 'detect' subcommands")

	detectCmd := flag.NewFlagSet("detect", flag.ExitOnError)
	bumpCmd := flag.NewFlagSet("bump", flag.ExitOnError)
	versionFlag := flag.Bool("version", false, "print the version of this program")
	currentFlag := detectCmd.Bool("current", false, "print current nixpath without bumping")
	lastFlag := detectCmd.Bool("last", false, "print git head ref without bumping")
	target := detectCmd.Bool("target", false, "print which file will be bumped")
	flag.Parse()
	if *versionFlag {
		revision := commit[:7]
		fmt.Printf("%s\n", "nix-headbump"+" "+version+" "+"("+revision+") # "+date)
		return
	}

	if len(os.Args) < 2 {
		fmt.Println(usageError.Error())
		os.Exit(1)
	}

	path, err := getTargetPath()
	if err != nil {
		log.Fatalf("Failed to get target files: %s", err.Error())
	}
	if path == "" {
		log.Fatalln("Both default.nix and shell.nix are not found")
	}

	switch os.Args[1] {
	case "detect":
		err := detectCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(usageError.Error())
		}
		if *target {
			fmt.Println(path)
			return
		}
		if *currentFlag {
			current, err := getCurrentVersion(path)
			if err != nil {
				log.Fatalf("Getting the current version has been failed: %s", err.Error())
			}
			fmt.Println(current)
			return
		}
		last, err := getLastVersion()
		if err != nil {
			log.Fatalf("Getting the last version has been failed: %s", err.Error())
		}
		if *lastFlag {
			fmt.Println(last)
			return
		}

		detectCmd.PrintDefaults()
	case "bump":
		err := bumpCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(usageError.Error())
		}
		last, err := getLastVersion()
		if err != nil {
			log.Fatalf("Getting the last version has been failed: %s", err.Error())
		}
		if err = bump(path, last); err != nil {
			log.Fatalf("Bumping the version has been failed: %s", err.Error())
		}
	default:
		fmt.Println(usageError.Error())
		os.Exit(1)
	}
}

func bump(path string, last string) error {
	origin, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	replaced := re.ReplaceAll(origin, []byte("${1}"+last+"${3}"))
	if bytes.Equal(origin, replaced) {
		return nil
	}

	return os.WriteFile(path, replaced, os.ModePerm)
}

func getTargetPath() (string, error) {
	paths := [2]string{"default.nix", "shell.nix"}
	for _, path := range paths {
		_, err := os.Stat(path)
		if err == nil {
			return path, nil
		}

		if !os.IsNotExist(err) {
			return "", fmt.Errorf("Can not open %s: %w", path, err)
		}
	}
	return "", fmt.Errorf("%v are not found", paths)
}

func getCurrentVersion(path string) (string, error) {
	origin, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	matches := re.FindStringSubmatch(string(origin))
	return matches[2], nil
}

func getLastVersion() (string, error) {
	type commit struct {
		Sha string `json:"sha"`
	}

	type response struct {
		Commit commit `json:"commit"`
	}

	// https://docs.github.com/en/rest/branches/branches?apiVersion=2022-11-28#get-a-branch
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/NixOS/nixpkgs/branches/master", nil)
	// May be necessary to set "Authorization" header if frequent requests are needed.
	// -H "Authorization: Bearer <YOUR-TOKEN>"\
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := new(http.Client)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	jsonRes := &response{}
	if json.Unmarshal(body, jsonRes) != nil {
		return "", err
	}
	return jsonRes.Commit.Sha, nil
}
