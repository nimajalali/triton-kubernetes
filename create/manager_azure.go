package create

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joyent/triton-kubernetes/state"
	homedir "github.com/mitchellh/go-homedir"

	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/Azure/azure-sdk-for-go/arm/resources/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/manifoldco/promptui"
	"github.com/spf13/viper"
)

const (
	azureRancherTerraformModulePath = "terraform/modules/azure-rancher"
)

// This struct represents the definition of a Terraform .tf file.
// Marshalled into json this struct can be passed directly to Terraform.
type azureManagerTerraformConfig struct {
	baseManagerTerraformConfig

	AzureSubscriptionID    string `json:"azure_subscription_id"`
	AzureClientID          string `json:"azure_client_id"`
	AzureClientSecret      string `json:"azure_client_secret"`
	AzureTenantID          string `json:"azure_tenant_id"`
	AzureEnvironment       string `json:"azure_environment"`
	AzureLocation          string `json:"azure_location"`
	AzureResourceGroupName string `json:"azure_resource_group_name"`

	AzureSize           string `json:"azure_size"`
	AzureImagePublisher string `json:"azure_image_publisher,omitempty"`
	AzureImageOffer     string `json:"azure_image_offer,omitempty"`
	AzureImageSKU       string `json:"azure_image_sku,omitempty"`
	AzureImageVersion   string `json:"azure_image_version,omitempty"`

	AzureSSHUser        string `json:"azure_ssh_user"`
	AzurePublicKeyPath  string `json:"azure_public_key_path"`
	AzurePrivateKeyPath string `json:"azure_private_key_path"`
}

func newAzureManager(currentState state.State, name string) error {
	nonInteractiveMode := viper.GetBool("non-interactive")

	baseConfig, err := getBaseManagerTerraformConfig(azureRancherTerraformModulePath, name)
	if err != nil {
		return err
	}

	cfg := azureManagerTerraformConfig{
		baseManagerTerraformConfig: baseConfig,
	}

	// Azure Subscription ID
	if viper.IsSet("azure_subscription_id") {
		cfg.AzureSubscriptionID = viper.GetString("azure_subscription_id")
	} else if nonInteractiveMode {
		return errors.New("azure_subscription_id must be specified")
	} else {
		prompt := promptui.Prompt{
			Label: "Azure Subscription ID",
			Validate: func(input string) error {
				if len(input) == 0 {
					return errors.New("Invalid Azure Subscription ID")
				}
				return nil
			},
		}

		result, err := prompt.Run()
		if err != nil {
			return err
		}
		cfg.AzureSubscriptionID = result
	}

	// Azure Client ID
	if viper.IsSet("azure_client_id") {
		cfg.AzureClientID = viper.GetString("azure_client_id")
	} else if nonInteractiveMode {
		return errors.New("azure_client_id must be specified")
	} else {
		prompt := promptui.Prompt{
			Label: "Azure Client ID",
			Validate: func(input string) error {
				if len(input) == 0 {
					return errors.New("Invalid Azure Client ID")
				}
				return nil
			},
		}

		result, err := prompt.Run()
		if err != nil {
			return err
		}
		cfg.AzureClientID = result
	}

	// Azure Client Secret
	if viper.IsSet("azure_client_secret") {
		cfg.AzureClientSecret = viper.GetString("azure_client_secret")
	} else if nonInteractiveMode {
		return errors.New("azure_client_secret must be specified")
	} else {
		prompt := promptui.Prompt{
			Label: "Azure Client Secret",
			Validate: func(input string) error {
				if len(input) == 0 {
					return errors.New("Invalid Azure Client Secret")
				}
				return nil
			},
		}

		result, err := prompt.Run()
		if err != nil {
			return err
		}
		cfg.AzureClientSecret = result
	}

	// Azure Tenant ID
	if viper.IsSet("azure_tenant_id") {
		cfg.AzureTenantID = viper.GetString("azure_tenant_id")
	} else if nonInteractiveMode {
		return errors.New("azure_tenant_id must be specified")
	} else {
		prompt := promptui.Prompt{
			Label: "Azure Tenant ID",
			Validate: func(input string) error {
				if len(input) == 0 {
					return errors.New("Invalid Azure Tenant ID")
				}
				return nil
			},
		}

		result, err := prompt.Run()
		if err != nil {
			return err
		}
		cfg.AzureTenantID = result
	}

	// Azure Environment
	if viper.IsSet("azure_environment") {
		cfg.AzureEnvironment = viper.GetString("azure_environment")
	} else if nonInteractiveMode {
		return errors.New("azure_environment must be specified")
	} else {
		prompt := promptui.Select{
			Label: "Azure Environment",
			Items: []string{"public", "government", "german", "china"},
			Templates: &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   fmt.Sprintf(`%s {{ . | underline }}`, promptui.IconSelect),
				Inactive: `  {{ . }}`,
				Selected: fmt.Sprintf(`{{ "%s" | green }} {{ "Azure Environment:" | bold}} {{ . }}`, promptui.IconGood),
			},
		}

		_, value, err := prompt.Run()
		if err != nil {
			return err
		}

		cfg.AzureEnvironment = value
	}

	// Verify selected azure environment is valid
	if cfg.AzureEnvironment != "public" && cfg.AzureEnvironment != "government" && cfg.AzureEnvironment != "german" && cfg.AzureEnvironment != "china" {
		return fmt.Errorf("Invalid azure_environment '%s', must be one of the following: 'public', 'government', 'german', or 'china'", cfg.AzureEnvironment)
	}

	// Terraform expects public/government/german/china for azure environment
	// Azure SDK expects `Azure{Environment}Cloud`
	azureEnv, err := azure.EnvironmentFromName(fmt.Sprintf("Azure%sCloud", cfg.AzureEnvironment))
	if err != nil {
		return err
	}

	// We now have enough information to init an azure client
	oauthConfig, err := adal.NewOAuthConfig(azureEnv.ActiveDirectoryEndpoint, cfg.AzureTenantID)
	if err != nil {
		return err
	}

	azureSPT, err := adal.NewServicePrincipalToken(*oauthConfig, cfg.AzureClientID, cfg.AzureClientSecret, azureEnv.ResourceManagerEndpoint)
	if err != nil {
		return err
	}

	azureGroupClient := subscriptions.NewGroupClientWithBaseURI(azureEnv.ResourceManagerEndpoint)
	azureGroupClient.Authorizer = autorest.NewBearerAuthorizer(azureSPT)

	azureRawLocations, err := azureGroupClient.ListLocations(cfg.AzureSubscriptionID)
	if err != nil {
		return err
	}

	azureLocations := []string{}
	for _, loc := range *azureRawLocations.Value {
		azureLocations = append(azureLocations, *loc.DisplayName)
	}

	// Azure Location
	if viper.IsSet("azure_location") {
		cfg.AzureLocation = viper.GetString("azure_location")

		// Verify selected azure location exists
		found := false
		for _, location := range azureLocations {
			if cfg.AzureLocation == location {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Invalid azure_location '%s', must be one of the following: %s", cfg.AzureLocation, strings.Join(azureLocations, ", "))
		}
	} else if nonInteractiveMode {
		return errors.New("azure_location must be specified")
	} else {
		prompt := promptui.Select{
			Label: "Azure Location",
			Items: azureLocations,
			Searcher: func(input string, index int) bool {
				name := strings.Replace(strings.ToLower(azureLocations[index]), " ", "", -1)
				input = strings.Replace(strings.ToLower(input), " ", "", -1)
				return strings.Contains(name, input)
			},
			Templates: &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   fmt.Sprintf(`%s {{ . | underline }}`, promptui.IconSelect),
				Inactive: `  {{ . }}`,
				Selected: fmt.Sprintf(`{{ "%s" | green }} {{ "Azure Location:" | bold}} {{ . }}`, promptui.IconGood),
			},
		}

		_, value, err := prompt.Run()
		if err != nil {
			return err
		}

		cfg.AzureLocation = value
	}

	azureVMSizesClient := compute.NewVirtualMachineSizesClientWithBaseURI(azureEnv.ResourceManagerEndpoint, cfg.AzureSubscriptionID)
	azureVMSizesClient.Authorizer = autorest.NewBearerAuthorizer(azureSPT)

	azureRawVMSizes, err := azureVMSizesClient.List(strings.Replace(strings.ToLower(cfg.AzureLocation), " ", "", -1))
	if err != nil {
		return err
	}

	azureVMSizes := []string{}
	for _, size := range *azureRawVMSizes.Value {
		azureVMSizes = append(azureVMSizes, *size.Name)
	}

	// Azure Size
	if viper.IsSet("azure_size") {
		cfg.AzureSize = viper.GetString("azure_size")

		// Verify selected azure size exists
		found := false
		for _, size := range azureVMSizes {
			if cfg.AzureSize == size {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Invalid azure_size '%s', must be one of the following: %s", cfg.AzureSize, strings.Join(azureVMSizes, ", "))
		}
	} else if nonInteractiveMode {
		return errors.New("azure_size must be specified")
	} else {
		prompt := promptui.Select{
			Label: "Azure Size",
			Items: azureVMSizes,
			Searcher: func(input string, index int) bool {
				name := strings.Replace(strings.ToLower(azureVMSizes[index]), " ", "", -1)
				input = strings.Replace(strings.ToLower(input), " ", "", -1)
				return strings.Contains(name, input)
			},
			Templates: &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   fmt.Sprintf(`%s {{ . | underline }}`, promptui.IconSelect),
				Inactive: `  {{ . }}`,
				Selected: fmt.Sprintf(`{{ "%s" | green }} {{ "Azure Size:" | bold}} {{ . }}`, promptui.IconGood),
			},
		}

		_, value, err := prompt.Run()
		if err != nil {
			return err
		}

		cfg.AzureSize = value
	}

	azureImagesClient := compute.NewVirtualMachineImagesClientWithBaseURI(azureEnv.ResourceManagerEndpoint, cfg.AzureSubscriptionID)
	azureImagesClient.Authorizer = autorest.NewBearerAuthorizer(azureSPT)

	// imageResults, err := azureImagesClient.List("westus", "Canonical", "UbuntuServer", "16.04-LTS", "", nil, "")
	// if err != nil {
	// 	return []string{}, err
	// }

	// for _, x := range *imageResults.Value {
	// 	fmt.Println(*x.Name)
	// }

	// cfg.AzureImagePublisher = "Canonical"
	// cfg.AzureImageOffer = "UbuntuServer"
	// cfg.AzureImageSKU = "16.04-LTS"
	// cfg.AzureImageVersion = ""

	// Azure SSH User
	if viper.IsSet("azure_ssh_user") {
		cfg.AzureSSHUser = viper.GetString("azure_ssh_user")
	} else if nonInteractiveMode {
		return errors.New("azure_ssh_user must be specified")
	} else {
		prompt := promptui.Prompt{
			Label:   "Azure SSH User",
			Default: "ubuntu",
		}

		result, err := prompt.Run()
		if err != nil {
			return err
		}
		cfg.AzureSSHUser = result
	}

	// Azure Public Key Path
	if viper.IsSet("azure_public_key_path") {
		expandedPublicKeyPath, err := homedir.Expand(viper.GetString("azure_public_key_path"))
		if err != nil {
			return err
		}

		cfg.AzurePublicKeyPath = expandedPublicKeyPath

	} else if nonInteractiveMode {
		return errors.New("azure_public_key_path must be specified")
	} else {
		prompt := promptui.Prompt{
			Label: "Azure Public Key Path",
			Validate: func(input string) error {
				expandedPath, err := homedir.Expand(input)
				if err != nil {
					return err
				}

				_, err = os.Stat(expandedPath)
				if err != nil {
					if os.IsNotExist(err) {
						return errors.New("File not found")
					}
				}
				return nil
			},
			Default: "~/.ssh/id_rsa.pub",
		}

		result, err := prompt.Run()
		if err != nil {
			return err
		}

		expandedPublicKeyPath, err := homedir.Expand(result)
		if err != nil {
			return err
		}

		cfg.AzurePublicKeyPath = expandedPublicKeyPath
	}

	// Azure Private Key Path
	if viper.IsSet("azure_private_key_path") {
		expandedPrivateKeyPath, err := homedir.Expand(viper.GetString("azure_private_key_path"))
		if err != nil {
			return err
		}

		cfg.AzurePrivateKeyPath = expandedPrivateKeyPath

	} else if nonInteractiveMode {
		return errors.New("azure_private_key_path must be specified")
	} else {
		prompt := promptui.Prompt{
			Label: "Azure Private Key Path",
			Validate: func(input string) error {
				expandedPath, err := homedir.Expand(input)
				if err != nil {
					return err
				}

				_, err = os.Stat(expandedPath)
				if err != nil {
					if os.IsNotExist(err) {
						return errors.New("File not found")
					}
				}
				return nil
			},
			Default: "~/.ssh/id_rsa",
		}

		result, err := prompt.Run()
		if err != nil {
			return err
		}

		expandedPrivateKeyPath, err := homedir.Expand(result)
		if err != nil {
			return err
		}

		cfg.AzurePrivateKeyPath = expandedPrivateKeyPath
	}

	currentState.SetManager(&cfg)

	return nil
}
