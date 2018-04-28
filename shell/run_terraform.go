package shell

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/joyent/triton-kubernetes/state"

	"github.com/spf13/viper"
)

func RunTerraformApplyWithState(state state.State) error {
	// Create a temporary directory
	tempDir, err := ioutil.TempDir("", "triton-kubernetes-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// Save the terraform config to the temporary directory
	jsonPath := fmt.Sprintf("%s/%s", tempDir, "main.tf.json")
	err = ioutil.WriteFile(jsonPath, state.Bytes(), 0644)
	if err != nil {
		return err
	}

	// Use temporary directory as working directory
	shellOptions := ShellOptions{
		WorkingDir: tempDir,
	}

	// Run terraform init
	err = RunShellCommand(&shellOptions, GetTerraformCmd(), "init", "-force-copy")
	if err != nil {
		return err
	}

	// Run terraform apply
	err = RunShellCommand(&shellOptions, GetTerraformCmd(), "apply", "-auto-approve")
	if err != nil {
		return err
	}

	return nil
}

func RunTerraformDestroyWithState(currentState state.State, args []string) error {
	// Create a temporary directory
	tempDir, err := ioutil.TempDir("", "triton-kubernetes-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// Save the terraform config to the temporary directory
	jsonPath := fmt.Sprintf("%s/%s", tempDir, "main.tf.json")
	err = ioutil.WriteFile(jsonPath, currentState.Bytes(), 0644)
	if err != nil {
		return err
	}

	// Use temporary directory as working directory
	shellOptions := ShellOptions{
		WorkingDir: tempDir,
	}

	// Run terraform init
	err = RunShellCommand(&shellOptions, GetTerraformCmd(), "init", "-force-copy")
	if err != nil {
		return err
	}

	// Run terraform destroy
	allArgs := append([]string{"destroy", "-force"}, args...)
	err = RunShellCommand(&shellOptions, GetTerraformCmd(), allArgs...)
	if err != nil {
		return err
	}

	return nil
}

// Returns the command to use to run terraform.
// Returns the value of the terraform_binary config variable.
// If that's not set, returns "terraform".
func GetTerraformCmd() string {
	if viper.IsSet("terraform-binary") {
		return viper.GetString("terraform-binary")
	}
	return "terraform"
}
