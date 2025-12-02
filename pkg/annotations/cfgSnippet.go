package annotations

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-test/deep"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

//nolint:golint,stylecheck
const (
	COMMENT_CFG_SNIPPET_END   = "  ###_config-snippet_### END"
	COMMENT_CFG_SNIPPET_BEGIN = "  ###_config-snippet_### BEGIN"
	BACKEND                   = "backend"
	COMMMENT_INGRESS_PREFIX   = "### ingress:"
	COMMENT_ENDING            = " ###"
	COMMENT_CONFIGMAP_PREFIX  = "### configmap:"
	COMMMENT_SERVICE_PREFIX   = "### service:"
	SERVICE_NAME_PREFIX       = "svc_"
	INGRESS_NAME_PREFIX       = "ing_"
)

type CfgSnippet struct {
	ingress  *store.Ingress
	service  *store.Service
	name     string
	frontend string
	backend  string
}

type cfgData struct {
	status        store.Status
	value         []string
	updated       []string
	orderPriority int
	disabled      bool
}

// CfgSnippetType represents type of a config snippet
type CfgSnippetType string

const (
	// CfgSnippetType values
	ConfigSnippetBackend  CfgSnippetType = "backend"
	ConfigSnippetFrontend CfgSnippetType = "frontend"
	ConfigSnippetGlobal   CfgSnippetType = "global"
)

// cfgSnippet is a particular type of config that is not
// handled by the upstram library haproxytech/client-native.
// Which means there is no client-native models to
// store, exchange and query cfgSnippet Data. Thus this logic
// is directly handled by Ingress Controller in this package.
//
// The code in this file need to be rewritten to avoid init,
// global variables and rather expose a clean interface.
var cfgSnippet struct {
	global           *cfgData
	frontends        map[string]*cfgData
	frontendsCustom  map[string]map[string]*cfgData // [frontend][origin] = &cfgData{}
	backends         map[string]map[string]*cfgData // backends[backend][origin] = &cfgData{}
	disabledServices map[string]bool
	// Flags to allow disable some config snippet ("backend", "frontend", "global")
	disabledSnippets map[CfgSnippetType]struct{}
}

func init() { //nolint:gochecknoinits
	InitCfgSnippet()
}

func InitCfgSnippet() {
	cfgSnippet.global = &cfgData{}
	cfgSnippet.frontends = make(map[string]*cfgData)
	cfgSnippet.frontendsCustom = make(map[string]map[string]*cfgData)
	cfgSnippet.backends = make(map[string]map[string]*cfgData)
	cfgSnippet.disabledServices = make(map[string]bool)
	cfgSnippet.disabledSnippets = make(map[CfgSnippetType]struct{})
}

type ConfigSnippetOptions struct {
	Backend  *string
	Frontend *string
	Ingress  *store.Ingress
	Name     string
}

// DisableConfigSnippets fills a map[cfgSnippetType]struct{} of disabled config snippet types:
// - backend/frontend/global
// and store it in the global var cfgSnippet
// from a comma separated list : all,backend,frontend,global.
// If "all" is present in the list, then: backend, frontend and global config snippets are disabled.
func DisableConfigSnippets(disableConfigSnippets string) {
	disable := map[CfgSnippetType]struct{}{}
	if disableConfigSnippets != "" {
		for _, d := range strings.Split(disableConfigSnippets, ",") {
			switch strings.TrimSpace(d) {
			case "all":
				disable[ConfigSnippetBackend] = struct{}{}
				disable[ConfigSnippetFrontend] = struct{}{}
				disable[ConfigSnippetGlobal] = struct{}{}
			case "frontend":
				disable[ConfigSnippetFrontend] = struct{}{}
			case "backend":
				disable[ConfigSnippetBackend] = struct{}{}
			case "global":
				disable[ConfigSnippetGlobal] = struct{}{}
			default:
				logger.Errorf("wrong config snippet type '%s' in disable-config-snippets arg in command line", d)
			}
		}
	}
	cfgSnippet.disabledSnippets = disable
}

func IsConfigSnippetDisabled(name CfgSnippetType) bool {
	_, disabled := cfgSnippet.disabledSnippets[name]
	return disabled
}

func NewCfgSnippet(opts ConfigSnippetOptions) *CfgSnippet {
	frontend := ""
	backend := ""
	if opts.Backend != nil {
		backend = *opts.Backend
	}
	if opts.Frontend != nil {
		frontend = *opts.Frontend
	}
	return &CfgSnippet{
		name:     opts.Name,
		frontend: frontend,
		backend:  backend,
		ingress:  opts.Ingress,
	}
}

func (a *CfgSnippet) GetName() string {
	return a.name
}

func (a *CfgSnippet) Process(k store.K8s, annotations ...map[string]string) error {
	switch {
	case a.frontend != "":
		validator, err := validators.Get()
		if err != nil {
			return fmt.Errorf("failed to get validator: %w", err)
		}
		customAnnotations := map[string]string{}
		for _, annotation := range annotations {
			for k, v := range annotation {
				if strings.HasPrefix(k, "frontend."+a.frontend+"."+validator.Prefix()) {
					key := strings.TrimPrefix(k, "frontend."+a.frontend+"."+validator.Prefix())
					customAnnotations[key] = v
				}
			}
		}
		err = processCustomAnnotationsFrontend(customAnnotations, a, validator)
		if err != nil {
			return err
		}
		if IsConfigSnippetDisabled(ConfigSnippetFrontend) {
			// frontend snippet is disabled, do not handle
			return nil
		}
		var data []string
		input := common.GetValue(a.GetName(), annotations...)
		if input != "" {
			data = strings.Split(strings.Trim(input, "\n"), "\n")
		}

		_, ok := cfgSnippet.frontends[a.frontend]
		if !ok {
			cfgSnippet.frontends[a.frontend] = &cfgData{}
		}
		updated := deep.Equal(cfgSnippet.frontends[a.frontend].value, data)
		if len(updated) != 0 {
			cfgSnippet.frontends[a.frontend].value = data
			cfgSnippet.frontends[a.frontend].updated = updated
		}

	case a.backend != "":
		validator, err := validators.Get()
		if err != nil {
			return fmt.Errorf("failed to get validator: %w", err)
		}
		anns, customAnnotations := common.GetValuesAndIndices(a.GetName(), "backend."+validator.Prefix(), annotations...)
		processCustomAnnotationsBackend(customAnnotations, a, validator)

		if IsConfigSnippetDisabled(ConfigSnippetBackend) {
			// backend snippet is disabled, do not handle
			return nil
		}

		// We don't want configmap value unless it's configmap being processed.
		// We detect that by name of the backend and indice of maps providing the value
		if a.backend == "configmap" {
			if anns[0] != "" {
				// Create comment section for configmap configsnippet
				origin := "configmap"
				comment := COMMENT_CONFIGMAP_PREFIX + k.ConfigMaps.Main.Namespace + "/" + k.ConfigMaps.Main.Name + COMMENT_ENDING
				data := strings.Split(strings.Trim(anns[0], "\n"), "\n")
				data = append([]string{comment}, data...)
				processConfigSnippet(a.backend, origin, data, 0)
			}
		} else {
			if a.service != nil && a.service.Name != "" && !a.service.Faked && anns[0] != "" {
				origin := a.service.Namespace + "/" + a.service.Name
				comment := COMMMENT_SERVICE_PREFIX + a.backend + "/" + origin + COMMENT_ENDING
				data := strings.Split(strings.Trim(anns[0], "\n"), "\n")
				data = append([]string{comment}, data...)
				processConfigSnippet(a.backend, SERVICE_NAME_PREFIX+origin, data, 0)
			}
			if a.ingress != nil && anns[1] != "" {
				origin := a.ingress.Namespace + "/" + a.ingress.Name
				comment := COMMMENT_INGRESS_PREFIX + a.backend + "/" + origin + COMMENT_ENDING
				data := strings.Split(strings.Trim(anns[1], "\n"), "\n")
				data = append([]string{comment}, data...)
				processConfigSnippet(a.backend, INGRESS_NAME_PREFIX+origin, data, 0)
			}
		}
	default:
		if IsConfigSnippetDisabled(ConfigSnippetGlobal) {
			// global snippet is disabled, do not handle
			return nil
		}
		var data []string
		input := common.GetValue(a.GetName(), annotations...)
		if input != "" {
			data = strings.Split(strings.Trim(input, "\n"), "\n")
		}

		updated := deep.Equal(cfgSnippet.global.value, data)
		if len(updated) != 0 {
			cfgSnippet.global.value = data
			cfgSnippet.global.updated = updated
		}
	}
	return nil
}

func UpdateGlobalCfgSnippet(api api.HAProxyClient) (updated []string, err error) {
	err = api.GlobalCfgSnippet(cfgSnippet.global.value)
	if err != nil {
		return updated, err
	}

	if len(cfgSnippet.global.updated) == 0 {
		return updated, err
	}

	updated = cfgSnippet.global.updated
	cfgSnippet.global.updated = nil
	return updated, err
}

func UpdateFrontendCfgSnippet(api api.HAProxyClient, frontends ...string) (updated []string, err error) {
	for _, ft := range frontends {
		data, ok := cfgSnippet.frontends[ft]
		customData, okCustom := cfgSnippet.frontendsCustom[ft]
		if !ok && !okCustom {
			continue
		}
		if okCustom {
			newData := make([]string, 0)
			for _, v := range customData {
				newData = append(newData, v.value...)
				updated = append(updated, v.updated...)
			}
			newData = append(newData, "### custom annotations end ###")
			data = &cfgData{value: append(newData, data.value...)}
		}

		err = api.FrontendCfgSnippetSet(ft, data.value)
		if err != nil {
			return updated, err
		}

		if len(updated) == 0 {
			continue
		}

		updated = append(updated, data.updated...)
		data.updated = nil
		cfgSnippet.frontends[ft] = data
		for k, v := range cfgSnippet.frontendsCustom[ft] {
			v.updated = nil
			cfgSnippet.frontendsCustom[ft][k] = v
		}
	}
	return updated, err
}

func CheckBackendConfigSnippetError(configErr error, cfgDir string) (rerun bool, err error) {
	// No error ? no configsnippet to disable.
	if configErr == nil {
		return rerun, err
	}
	file, lineNumbers, err := processConfigurationError(configErr)
	if err != nil {
		return rerun, err
	}
	// Read contents from failed configuration file
	file = filepath.Join(cfgDir, "failed", filepath.Base(file))
	contents, err := os.ReadFile(file)
	if err != nil {
		return rerun, err
	}

	rerun = disableFaultyCfgSnippet(string(contents), lineNumbers)
	return rerun, err
}

func CheckBackendConfigSnippetErrorOnReload(configErr error, cfgDir string) (rerun bool, err error) {
	// No error ? no configsnippet to disable.
	if configErr == nil {
		return rerun, err
	}
	file, lineNumbers, err := processConfigurationError(configErr)
	if err != nil {
		return rerun, err
	}
	// Read contents from failed configuration file
	file = filepath.Join(cfgDir, filepath.Base(file))
	contents, err := os.ReadFile(file)
	if err != nil {
		return rerun, err
	}

	rerun = disableFaultyCfgSnippet(string(contents), lineNumbers)
	return rerun, err
}

func RemoveBackendCfgSnippet(backend string) {
	if cfgSnippet.backends == nil {
		return
	}
	delete(cfgSnippet.backends, backend)
}

func (a *CfgSnippet) SetService(service *store.Service) {
	a.service = service
}

func processConfigSnippetFrontendCustom(frontend, origin string, data []string, orderPriority int) {
	var exists bool
	if _, exists = cfgSnippet.frontendsCustom[frontend][origin]; !exists {
		// Prevent empty configsnippet to be inserted (with only comment)
		// and if no data is provided
		if len(data) == 1 || data == nil {
			return
		}
		cfgSnippet.frontendsCustom[frontend][origin] = &cfgData{status: store.ADDED, orderPriority: orderPriority}
	}

	currentCfgData := cfgSnippet.frontendsCustom[frontend][origin]
	// As reseen it's not to be deleted
	if currentCfgData.status == store.DELETED {
		currentCfgData.status = store.EMPTY
	}

	updated := deep.Equal(currentCfgData.value, data)
	// Something changed from possibly existing configsnippet value ?
	// If new configsnippet this would generate a difference between empty and something.
	if len(updated) != 0 {
		// A change so update.
		currentCfgData.value = data
		currentCfgData.updated = updated
		currentCfgData.orderPriority = orderPriority
		if exists {
			// as existing, set status to modified and reset disable status as now should be retested.
			currentCfgData.status = store.MODIFIED
			currentCfgData.disabled = false
		}
	}
}

func processConfigSnippet(backend, origin string, data []string, orderPriority int) {
	var exists bool
	if _, exists = cfgSnippet.backends[backend][origin]; !exists {
		// Prevent empty configsnippet to be inserted (with only comment)
		// and if no data is provided
		if len(data) == 1 || data == nil {
			return
		}
		cfgSnippet.backends[backend][origin] = &cfgData{status: store.ADDED, orderPriority: orderPriority}
	}

	currentCfgData := cfgSnippet.backends[backend][origin]
	// As reseen it's not to be deleted
	if currentCfgData.status == store.DELETED {
		currentCfgData.status = store.EMPTY
	}

	updated := deep.Equal(currentCfgData.value, data)
	// Something changed from possibly existing configsnippet value ?
	// If new configsnippet this would generate a difference between empty and something.
	if len(updated) != 0 {
		// A change so update.
		currentCfgData.value = data
		currentCfgData.updated = updated
		currentCfgData.orderPriority = orderPriority
		if exists {
			// as existing, set status to modified and reset disable status as now should be retested.
			currentCfgData.status = store.MODIFIED
			currentCfgData.disabled = false
		}
	}
}

func getErrorLineNumberAndFileName(msg string) (lineNumber int, file string, err error) {
	lineNumber = -1
	openSquareBracket := strings.Index(msg, "[")
	if openSquareBracket == -1 {
		return lineNumber, file, err
	}
	closeSquareBracket := strings.Index(msg, "]")
	if closeSquareBracket == -1 {
		return lineNumber, file, err
	}
	configsnippetComment := msg[openSquareBracket+1 : closeSquareBracket]
	fileLineNumber := strings.Split(configsnippetComment, ":")
	// The error line number and file name of configuration file is in format [file:line number] in the reporting error line
	if len(fileLineNumber) == 2 {
		file = fileLineNumber[0]
		lineNumber, err = strconv.Atoi(fileLineNumber[1])
		if err != nil {
			return lineNumber, file, err
		}
	} else {
		err = fmt.Errorf("invalid config snippet information : '%s'", configsnippetComment)
	}
	return lineNumber, file, err
}

func disableFaultyCfgSnippet(contents string, lineNumbers []int) (rerun bool) {
	configLines := strings.Split(contents, "\n")
	// Start parsing the configuration file to find corresponding configsnippet identification comment.
	for _, lineNumber := range lineNumbers {
		configSnippet := ""
		globalConfigSnippet := ""
		svcConfigSnippet := ""
		// From error line number we iterate towards top of the file
		for i := lineNumber - 1; i >= 0; i-- {
			line := configLines[i]
			// If we reach a boundary of config snippet comment
			// or if we reach backend section declaration we're finished with this line number
			if line == COMMENT_CFG_SNIPPET_END ||
				line == COMMENT_CFG_SNIPPET_BEGIN ||
				strings.HasPrefix(line, BACKEND) {
				break
			}
			// If we reach a comment for ingress config snippet infos, we can extract them
			if strings.HasPrefix(strings.TrimLeft(line, " "), COMMMENT_INGRESS_PREFIX) &&
				strings.HasSuffix(strings.TrimLeft(line, " "), COMMENT_ENDING) {
				configSnippet = line[len(COMMMENT_INGRESS_PREFIX)+2 : len(line)-len(COMMENT_ENDING)]
				break
			}
			// If we reach a comment for configmap config snippet infos, we can extract them
			if strings.HasPrefix(strings.TrimLeft(line, " "), COMMENT_CONFIGMAP_PREFIX) &&
				strings.HasSuffix(line, COMMENT_ENDING) {
				globalConfigSnippet = line[len(COMMENT_CONFIGMAP_PREFIX)+2 : len(line)-len(COMMENT_ENDING)]
				break
			}
			// If we reach a comment for service config snippet infos, we can extract them
			if strings.HasPrefix(strings.TrimLeft(line, " "), COMMMENT_SERVICE_PREFIX) &&
				strings.HasSuffix(line, COMMENT_ENDING) {
				svcConfigSnippet = line[len(COMMMENT_SERVICE_PREFIX)+2 : len(line)-len(COMMENT_ENDING)]
				break
			}
		}
		// we disable corresponding config snippet
		if configSnippet != "" {
			tokens := strings.SplitN(configSnippet, "/", 2)
			backend, configSnippetName := tokens[0], tokens[1]
			cfgSnippet.backends[backend][INGRESS_NAME_PREFIX+configSnippetName].disabled = true
			rerun = true
		}
		// we disable corresponding config snippet
		if svcConfigSnippet != "" {
			tokens := strings.SplitN(svcConfigSnippet, "/", 2)
			backend, configSnippetName := tokens[0], tokens[1]
			cfgSnippet.backends[backend][SERVICE_NAME_PREFIX+configSnippetName].disabled = true
			rerun = true
		}
		// We reserve the special backend "configmap" for configmap config snippet
		if globalConfigSnippet != "" {
			cfgSnippet.backends["configmap"]["configmap"].disabled = true
			rerun = true
		}
	}
	return rerun
}

func processConfigurationError(configErr error) (file string, lineNumbers []int, err error) {
	// Break error message into lines
	msgs := strings.Split(configErr.Error(), "\n")
	// storage of errors lines numbers
	lineNumbers = []int{}
	// Parse error message line to get lines in error in configuration file.
	for _, msg := range msgs {
		var lineNumber int
		var fileInError string
		if lineNumber, fileInError, err = getErrorLineNumberAndFileName(msg); err == nil && lineNumber >= 0 {
			if file == "" && fileInError != "" {
				file = fileInError
			}
			lineNumbers = append(lineNumbers, lineNumber)
		} else if err != nil {
			return file, lineNumbers, err
		}
	}
	return file, lineNumbers, err
}
