package e2e

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	WaitDuration = 60 * time.Second
	TickDuration = 2 * time.Second
)

var (
	devModeFlag = flag.Bool("dev", false, "keep test environment after finishing")
	devMode     bool
)

type Test struct {
	namespace     string
	templateDir   string
	tearDownFuncs []TearDownFunc
}

type TearDownFunc func() error

func NewTest() (test Test, err error) {
	flag.Parse()
	devMode = *devModeFlag
	test = Test{}
	// Namespace
	if test.namespace, err = setupNamespace(); err != nil {
		return test, fmt.Errorf("error setting test namespace: %w", err)
	}
	deleteNamespace(test.namespace, true)
	_, err = test.execute("", "kubectl", "create", "ns", test.namespace)
	if err != nil {
		return test, fmt.Errorf("error creating test namespace : %w ", err)
	}
	// TearDownFuncs
	test.tearDownFuncs = []TearDownFunc{func() error {
		deleteNamespace(test.namespace, false)
		return nil
	}}
	// TemplateDir
	test.templateDir, err = os.MkdirTemp("/tmp/", "haproxy-ic-test-tmpl")
	if err != nil {
		return test, fmt.Errorf("error creating template dir: %w ", err)
	}
	return test, nil
}

func (t *Test) GetNS() string {
	return t.namespace
}

func (t *Test) Apply(path string, namespace string, tmplData interface{}) error {
	var err error
	var file []byte
	if tmplData != nil {
		if path, err = t.processTemplate(path, tmplData); err != nil {
			return err
		}
	}
	if file, err = os.ReadFile(path); err != nil {
		return fmt.Errorf("error reading yaml file: %w", err)
	}
	// kubectl -n $NS apply -f -
	if out, errApply := t.execute(string(file), "kubectl", "-n", namespace, "apply", "-f", "-"); errApply != nil {
		return fmt.Errorf("error applying yaml file: %s", out)
	}
	return nil
}

func (t *Test) processTemplate(path string, tmplData interface{}) (string, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error reading yaml template: %w", err)
	}
	var result bytes.Buffer
	tmpl := template.Must(template.New("").Parse(string(file)))
	err = tmpl.Execute(&result, tmplData)
	if err != nil {
		return "", fmt.Errorf("error parsing yaml template: %w", err)
	}
	yaml := filepath.Join(t.templateDir, t.namespace+time.Now().Format("2006-01-02-1504051111")+".yaml")
	return yaml, os.WriteFile(yaml, result.Bytes(), 0o600)
}

func (t *Test) Delete(path string) error {
	var err error
	var file []byte
	if file, err = os.ReadFile(path); err != nil {
		return fmt.Errorf("error reading yaml file: %w", err)
	}
	if out, errApply := t.execute(string(file), "kubectl", "delete", "-f", "-"); errApply != nil {
		err = fmt.Errorf("error applying yaml file: %s", out)
	}
	return err
}

func (t *Test) TearDown() error {
	if devMode {
		return nil
	}
	os.RemoveAll(t.templateDir)
	for _, f := range t.tearDownFuncs {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

func (t *Test) AddTearDown(teardown TearDownFunc) {
	t.tearDownFuncs = append(t.tearDownFuncs, teardown)
}

func (t *Test) GetK8sVersion() (major, minor int, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, 0, err
	}
	config, err := clientcmd.BuildConfigFromFlags("", fmt.Sprintf("%s/.kube/config", home))
	if err != nil {
		return 0, 0, err
	}
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return 0, 0, err
	}
	version, _ := cs.DiscoveryClient.ServerVersion()
	major, _ = strconv.Atoi(version.Major)
	minor, _ = strconv.Atoi(version.Minor)
	return major, minor, nil
}

func (t *Test) execute(entry, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	var b bytes.Buffer
	b.WriteString(entry)
	cmd.Stdin = &b
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func deleteNamespace(namespace string, newSetup bool) {
	if devMode && !newSetup {
		return
	}
	deleteCmd := exec.Command("kubectl", "delete", "namespace", namespace)
	_ = deleteCmd.Run()
}

func setupNamespace() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir = filepath.Base(dir)
	dir = strings.Map(func(r rune) rune {
		if r < 'a' || r > 'z' && r != '-' {
			return '-'
		}
		return r
	}, strings.ToLower(dir))
	return "e2e-tests-" + dir, nil
}
