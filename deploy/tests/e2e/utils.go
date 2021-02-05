package e2e

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

var WaitDuration = 60 * time.Second
var TickDuration = 2 * time.Second

var devModeFlag = flag.Bool("dev", false, "keep test environmnet after finishing")
var devMode bool

type Test struct {
	namespace     string
	tearDownFuncs []TearDownFunc
	templateDir   string
}

type TearDownFunc func() error

func NewTest() (test Test, err error) {
	flag.Parse()
	devMode = *devModeFlag
	test = Test{}
	// Namespace
	if test.namespace, err = setupNamespace(); err != nil {
		return test, fmt.Errorf("error setting test namespace %s:", err)
	}
	deleteNamespace(test.namespace, true)
	test.execute("", "kubectl", "create", "ns", test.namespace)
	// TearDownFuncs
	test.tearDownFuncs = []TearDownFunc{func() error {
		deleteNamespace(test.namespace, false)
		return nil
	}}
	// TemplateDir
	test.templateDir, err = ioutil.TempDir("/tmp/", "haproxy-ic-test-tmpl")
	if err != nil {
		return test, fmt.Errorf("error creating template dir: %s ", err.Error())
	}
	return test, nil
}

func (t *Test) GetNS() string {
	return t.namespace
}

func (t *Test) DeployYaml(path string, namespace string) error {
	yaml, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading yaml file: %s", err)
	}
	// kubectl -n $NS apply -f -
	out, err := t.execute(string(yaml), "kubectl", "-n", namespace, "apply", "-f", "-")
	if err != nil {
		return fmt.Errorf("error applying yaml file: %s", out)
	}
	return nil
}

func (t *Test) DeployYamlTemplate(path string, namespace string, data interface{}) error {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading yaml template: %s", err)
	}
	var result bytes.Buffer
	tmpl := template.Must(template.New("").Parse(string(file)))
	err = tmpl.Execute(&result, data)
	if err != nil {
		return fmt.Errorf("error parsing yaml template: %s", err)
	}
	yaml := filepath.Join(t.templateDir, t.namespace+time.Now().Format("2006-01-02-1504051111")+".yaml")
	err = ioutil.WriteFile(yaml, result.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("error writing generated yaml template: %s", err)
	}
	return t.DeployYaml(yaml, namespace)
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
	deleteCmd.Run()
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
