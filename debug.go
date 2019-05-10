package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

func setupTestEnv() {
	log.Printf("Running in test env")
	err := os.MkdirAll(TestFolderPath, 0755)
	LogErr(err)
	HAProxyCFG = path.Join(TestFolderPath, HAProxyCFG)
	HAProxyGlobalCFG = path.Join(TestFolderPath, HAProxyGlobalCFG)
	HAProxyCertDir = path.Join(TestFolderPath, HAProxyCertDir)
	HAProxyStateDir = path.Join(TestFolderPath, HAProxyStateDir)
	cmd := exec.Command("pwd")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(dir)
	copyFile(path.Join(dir, "fs/etc/haproxy/haproxy.cfg"), HAProxyCFG)
	copyFile(path.Join(dir, "fs/etc/haproxy/global.cfg"), HAProxyGlobalCFG)
	log.Println(string(out))
}

func copyFile(src, dst string) {
	cmd := fmt.Sprintf("cp %s %s", src, dst)
	log.Println(cmd)
	result := exec.Command("bash", "-c", cmd)
	_, err := result.CombinedOutput()
	LogErr(err)
}
