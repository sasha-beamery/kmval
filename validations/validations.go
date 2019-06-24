package validations

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Common    map[string]*KubernetesObjectValidations `yaml:"common"`
	Artifacts map[string]*Artifact                    `yaml:"artifacts"`
}

type KubernetesObjectValidations struct {
	Object   string
	Defined  DefinedValidations `yaml:"defined"`
	Strings  StringValidations  `yaml:"strings"`
	Partials PartialValidations `yaml:"partials"`
	Integers IntegerValidations `yaml:"integers"`
}

type Artifact struct {
	Name      string
	TestPlans []TestPlan
	Base      KustomizeLayer  `yaml:"base"`
	Overlays  KustomizeLayers `yaml:"overlays"`
}

type KustomizeLayer map[string]*KubernetesObjectValidations
type KustomizeLayers map[string]KustomizeLayer

type DefinedValidations map[string]bool
type StringValidations map[string]string
type PartialValidations map[string]string
type IntegerValidations map[string]int

func LoadManifest(file string) (*Manifest, error) {
	var v Manifest
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(bytes, &v); err != nil {
		return nil, err
	}

	for object, kubernetesObjectValidations := range v.Common {
		kubernetesObjectValidations.Object = object
	}

	for name, artifact := range v.Artifacts {
		artifact.Name = name

		for object, kubernetesObjectValidations := range artifact.Base {
			kubernetesObjectValidations.Object = object
		}

		for _, kustomizeLayer := range artifact.Overlays {
			for object, kubernetesObjectValidations := range kustomizeLayer {
				kubernetesObjectValidations.Object = object
			}
		}
	}

	for object, kubernetesObject := range v.Common {
		kubernetesObject.Object = object
	}

	return &v, nil
}

func (a *Artifact) KustomizeBuild(overlay string) ([]byte, error) {
	cmd := exec.Command("kustomize", "build")
	cmd.Dir = filepath.Join(a.Name, overlay)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (a *Artifact) CreateTestPlans(commonValidations map[string]*KubernetesObjectValidations) error {
	plan, err := a.CreateTestPlan("base", commonValidations)
	if err != nil {
		return err
	}

	a.TestPlans = append(a.TestPlans, *plan)

	for overlay := range a.Overlays {
		plan, err := a.CreateTestPlan(overlay, commonValidations)
		if err != nil {
			return err
		}

		a.TestPlans = append(a.TestPlans, *plan)
	}

	return nil
}

func (a *Artifact) CreateTestPlan(overlay string, commonValidations map[string]*KubernetesObjectValidations) (*TestPlan, error) {
	plan := TestPlan{Name: a.Name, Overlay: overlay}
	bytes, err := a.KustomizeBuild(overlay)
	if err != nil {
		return nil, err
	}

	plan.KustomizeBuildYAMLDocuments = bytes
	plan.QueryToBoolExpectation = map[string]bool{}
	plan.QueryToIntegerExpectation = map[string]int{}
	plan.QueryToStringExpectation = map[string]string{}
	plan.QueryToKubernetesObject = map[string]string{}
	plan.KubernetesObjectToQuery = map[string][]string{}
	plan.QueryToValidationType = map[string]int{}

	registeredObjects := map[string]map[string]bool{}

	var registerKubernetesObjectToQuery = func(yqQuery string, kubernetesObject string) {
		if !registeredObjects[kubernetesObject][yqQuery] {
			plan.KubernetesObjectToQuery[kubernetesObject] = append(plan.KubernetesObjectToQuery[kubernetesObject], yqQuery)
			if registeredObjects[kubernetesObject] == nil {
				registeredObjects[kubernetesObject] = map[string]bool{}
			}

			registeredObjects[kubernetesObject][yqQuery] = true
		}
	}

	var definedMapper = func(yqQuery string, value bool, kubernetesObjectValidations *KubernetesObjectValidations) {
		plan.QueryToBoolExpectation[yqQuery] = value
		plan.QueryToKubernetesObject[yqQuery] = kubernetesObjectValidations.Object
		plan.QueryToValidationType[yqQuery] = Defined
		registerKubernetesObjectToQuery(yqQuery, kubernetesObjectValidations.Object)
	}

	var integerMapper = func(yqQuery string, value int, kubernetesObjectValidations *KubernetesObjectValidations) {
		plan.QueryToIntegerExpectation[yqQuery] = value
		plan.QueryToKubernetesObject[yqQuery] = kubernetesObjectValidations.Object
		plan.QueryToValidationType[yqQuery] = Integers
		registerKubernetesObjectToQuery(yqQuery, kubernetesObjectValidations.Object)
	}

	var stringMapper = func(yqQuery string, value string, kubernetesObjectValidations *KubernetesObjectValidations) {
		plan.QueryToStringExpectation[yqQuery] = value
		plan.QueryToKubernetesObject[yqQuery] = kubernetesObjectValidations.Object
		plan.QueryToValidationType[yqQuery] = Strings
		registerKubernetesObjectToQuery(yqQuery, kubernetesObjectValidations.Object)
	}

	var partialsMapper = func(yqQuery string, value string, kubernetesObjectValidations *KubernetesObjectValidations) {
		plan.QueryToStringExpectation[yqQuery] = value
		plan.QueryToKubernetesObject[yqQuery] = kubernetesObjectValidations.Object
		plan.QueryToValidationType[yqQuery] = Partials
		registerKubernetesObjectToQuery(yqQuery, kubernetesObjectValidations.Object)
	}

	for _, kubernetesObjectValidations := range commonValidations {
		for yqQuery, value := range kubernetesObjectValidations.Defined {
			definedMapper(yqQuery, value, kubernetesObjectValidations)
		}

		for yqQuery, value := range kubernetesObjectValidations.Integers {
			integerMapper(yqQuery, value, kubernetesObjectValidations)
		}

		for yqQuery, value := range kubernetesObjectValidations.Strings {
			stringMapper(yqQuery, value, kubernetesObjectValidations)
		}

		for yqQuery, value := range kubernetesObjectValidations.Partials {
			partialsMapper(yqQuery, value, kubernetesObjectValidations)
		}
	}

	for _, kubernetesObjectValidations := range a.Base {
		for yqQuery, value := range kubernetesObjectValidations.Defined {
			definedMapper(yqQuery, value, kubernetesObjectValidations)
		}

		for yqQuery, value := range kubernetesObjectValidations.Integers {
			integerMapper(yqQuery, value, kubernetesObjectValidations)
		}

		for yqQuery, value := range kubernetesObjectValidations.Strings {
			stringMapper(yqQuery, value, kubernetesObjectValidations)
		}

		for yqQuery, value := range kubernetesObjectValidations.Partials {
			partialsMapper(yqQuery, value, kubernetesObjectValidations)
		}
	}

	if overlay == "base" {
		return &plan, nil
	}

	for o, layer := range a.Overlays {
		if o == overlay {
			for _, kubernetesObjectValidations := range layer {
				for yqQuery, value := range kubernetesObjectValidations.Defined {
					definedMapper(yqQuery, value, kubernetesObjectValidations)
				}

				for yqQuery, value := range kubernetesObjectValidations.Integers {
					integerMapper(yqQuery, value, kubernetesObjectValidations)
				}

				for yqQuery, value := range kubernetesObjectValidations.Strings {
					stringMapper(yqQuery, value, kubernetesObjectValidations)
				}

				for yqQuery, value := range kubernetesObjectValidations.Partials {
					partialsMapper(yqQuery, value, kubernetesObjectValidations)
				}
			}
		}
	}

	return &plan, nil
}
