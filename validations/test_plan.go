package validations

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

const (
	Defined = iota
	Strings
	Integers
	Partials
)

type TestPlan struct {
	Name                                         string
	Overlay                                      string
	KubernetesObjectToKustomizeYAMLDocumentIndex map[string]int
	KubernetesObjects                            []string
	KubernetesObjectToQuery                      map[string][]string
	KustomizeBuildYAMLDocuments                  []byte
	QueryToBoolExpectation                       map[string]bool
	QueryToIntegerExpectation                    map[string]int
	QueryToKubernetesObject                      map[string]string
	QueryToStringExpectation                     map[string]string
	QueryToValidationType                        map[string]int
}

func Yq(kustomizeBuildYAMLDocuments []byte, documentNumber int, yqQuery string) (string, error) {
	yq := exec.Command("yq", "read", "-", "-d", strconv.Itoa(documentNumber), yqQuery)

	stdin := bytes.NewBuffer(kustomizeBuildYAMLDocuments)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	yq.Stdin = stdin
	yq.Stdout = &stdout
	yq.Stderr = &stderr

	if err := yq.Start(); err != nil {
		return "", err
	}

	if err := yq.Wait(); err != nil {
		fmt.Println(stderr.String())
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (t *TestPlan) Execute() (error, bool) {
	t.mapKubernetesObjectsToKustomizeYAMLDocumentIndices()

	fail := color.RedString("FAIL")
	pass := color.GreenString("PASS")

	var passes []string
	var failures []string

	for _, object := range t.KubernetesObjects {
		for _, query := range t.KubernetesObjectToQuery[object] {
			output, err := Yq(t.KustomizeBuildYAMLDocuments, t.KubernetesObjectToKustomizeYAMLDocumentIndex[object], query)
			if err != nil {
				return err, false
			}

			switch t.QueryToValidationType[query] {
			case Defined:
				expected := t.QueryToBoolExpectation[query]
				if output == "null" && expected == false {
					passes = append(passes, fmt.Sprintf("%s: %s %s %s\n", pass, object, query, color.GreenString("UNDEFINED")))
				}

				if output == "null" && expected == true {
					failures = append(failures, fmt.Sprintf("%s: %s %s %s\n", fail, object, query, color.RedString("UNDEFINED")))
				}

				if output != "null" && expected == true {
					passes = append(passes, fmt.Sprintf("%s: %s %s %s\n", pass, object, query, color.GreenString("DEFINED")))
				}

				if output != "null" && expected == false {
					failures = append(failures, fmt.Sprintf("%s: %s %s %s\n", fail, object, query, color.RedString("DEFINED")))
				}
			case Partials:
				expected := t.QueryToStringExpectation[query]
				if strings.Contains(output, expected) {
					passes = append(passes, fmt.Sprintf("%s: %s %s contains %s\n", pass, object, query, color.GreenString(expected)))
				} else {
					failures = append(failures, fmt.Sprintf("%s: %s %s does not contain %s \n", fail, object, query, color.RedString(expected)))
				}
			case Integers:
				expected := t.QueryToIntegerExpectation[query]
				if output == strconv.Itoa(expected) {
					passes = append(passes, fmt.Sprintf("%s: %s %s %s\n", pass, object, query, color.GreenString("%d", expected)))
				} else {
					failures = append(failures, fmt.Sprintf("%s: %s %s %s\n", fail, object, query, color.RedString("%d", expected)))
				}
			case Strings:
				expected := t.QueryToStringExpectation[query]
				if output == expected {
					passes = append(passes, fmt.Sprintf("%s: %s %s %s\n", pass, object, query, color.GreenString(expected)))
				} else {
					failures = append(failures, fmt.Sprintf("%s: %s %s %s \n", fail, object, query, color.RedString(expected)))
				}
			}
		}
	}

	if len(failures) == 0 {
		color.Green("%s/%s", t.Name, t.Overlay)
	} else {
		color.Red("\n%s/%s\n", t.Name, t.Overlay)

		sort.Strings(failures)
		for _, failure := range failures {
			fmt.Printf(failure)
		}

		fmt.Println()
	}

	return nil, len(failures) == 0
}

func (t *TestPlan) mapKubernetesObjectsToKustomizeYAMLDocumentIndices() {
	var documents []interface{}
	var document interface{}
	reader := bytes.NewReader(t.KustomizeBuildYAMLDocuments)
	decoder := yaml.NewDecoder(reader)

	for {
		if err := decoder.Decode(&document); err != nil {
			if err == io.EOF {
				break
			}
		} else {
			documents = append(documents, document)
		}
	}

	registeredObjects := map[string]bool{}

	var kubernetesObjects []string
	for _, object := range t.QueryToKubernetesObject {
		if !registeredObjects[object] {
			kubernetesObjects = append(kubernetesObjects, object)
			registeredObjects[object] = true

		}
	}

	for index, document := range documents {
		objectKind := document.(map[interface{}]interface{})["kind"]
		for _, object := range kubernetesObjects {
			if objectKind == object {
				if t.KubernetesObjectToKustomizeYAMLDocumentIndex == nil {
					t.KubernetesObjectToKustomizeYAMLDocumentIndex = map[string]int{}
				}

				t.KubernetesObjectToKustomizeYAMLDocumentIndex[object] = index
			}
		}
	}

	t.KubernetesObjects = kubernetesObjects
}
