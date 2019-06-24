package cli

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/LGUG2Z/kmval/validations"
	"github.com/fatih/color"
	"github.com/urfave/cli"
)

var (
	Version                      string
	Commit                       string
	ErrManifestValidationsFailed = fmt.Errorf(color.RedString("\nManifest validations failed!"))
	ErrEitherOneOrNoArguments    = fmt.Errorf("kmval takes either one or no arguments")
)

func App() *cli.App {
	app := cli.NewApp()

	app.Name = "kmval"
	app.Usage = "Kustomize Manifest Validator"
	app.EnableBashCompletion = true
	app.Compiled = time.Now()
	app.Version = Version
	app.Authors = []cli.Author{{
		Name:  "J. Iqbal",
		Email: "jade@beamery.com",
	}}

	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "fail-fast", Usage: "stop running validations after the first failure"},
		cli.StringFlag{Name: "file", Usage: "name of validations file", Value: "validations.yaml"},
	}

	var isInstalled = func(name string) bool {
		cmd := exec.Command("/bin/sh", "-c", "command -v "+name)
		if err := cmd.Run(); err != nil {
			return false
		}

		return true
	}

	app.Before = func(c *cli.Context) error {
		if !isInstalled("kustomize") {
			return fmt.Errorf("kmval requires kustomize to be installed and available in the $PATH")
		}

		if !isInstalled("yq") {
			return fmt.Errorf("kmval requires yq to be installed and available in the $PATH")
		}

		return nil
	}

	app.Action = cli.ActionFunc(func(c *cli.Context) error {
		switch c.NArg() {
		case 0:
		case 1:
			workingDirectory := c.Args().First()

			if err := os.Chdir(workingDirectory); err != nil {
				return err
			}
		default:
			return ErrEitherOneOrNoArguments
		}

		validationsManifest, err := validations.LoadManifest(c.String("file"))
		if err != nil {
			log.Fatal(err)
		}

		suitePass := true
		var sortedArtifacts []*validations.Artifact

		for _, artifact := range validationsManifest.Artifacts {
			sortedArtifacts = append(sortedArtifacts, artifact)
		}

		var sortArtifacts = func(i, j int) bool {
			return sortedArtifacts[i].Name < sortedArtifacts[j].Name
		}

		sort.Slice(sortedArtifacts, sortArtifacts)
		var failedPlans []string
		var passedPlans []string

		for _, artifact := range sortedArtifacts {
			if err = artifact.CreateTestPlans(validationsManifest.Common); err != nil {
				log.Fatal(err)
			}

			for _, testPlan := range artifact.TestPlans {
				err, pass := testPlan.Execute()
				if err != nil {
					log.Fatal(err)
				}

				if !pass {
					if c.Bool("fail-fast") {
						return ErrManifestValidationsFailed
					}

					suitePass = false
					failedPlans = append(failedPlans, fmt.Sprintf("%s/%s", testPlan.Name, testPlan.Overlay))
				} else {
					passedPlans = append(passedPlans, fmt.Sprintf("%s/%s", testPlan.Name, testPlan.Overlay))
				}
			}
		}

		switch suitePass {
		case false:
			fmt.Println()

			for _, plan := range passedPlans {
				color.Green("PASSED: %s", plan)
			}

			fmt.Println()

			for _, plan := range failedPlans {
				color.Red("FAILED: %s", plan)
			}

			return ErrManifestValidationsFailed
		default:
			color.Green("\nManifest validations passed!")
			return nil
		}
	})

	return app
}
