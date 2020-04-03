package scw

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"text/template"

	"github.com/scaleway/scaleway-sdk-go/internal/auth"
	"github.com/scaleway/scaleway-sdk-go/internal/errors"
	"github.com/scaleway/scaleway-sdk-go/logger"
	"gopkg.in/yaml.v2"
)

const (
	documentationLink       = "https://github.com/scaleway/scaleway-sdk-go/blob/master/scw/README.md"
	defaultConfigPermission = 0600
)

const configFileTemplate = `# Scaleway configuration file
# https://github.com/scaleway/scaleway-sdk-go/tree/master/scw#scaleway-config

# This configuration file can be used with:
# - Scaleway SDK Go (https://github.com/scaleway/scaleway-sdk-go)
# - Scaleway CLI (>2.0.0) (https://github.com/scaleway/scaleway-cli)
# - Scaleway Terraform Provider (https://www.terraform.io/docs/providers/scaleway/index.html)

# You need an access key and a secret key to connect to Scaleway API.
# Generate your token at the following address: https://console.scaleway.com/account/credentials

# An access key is a secret key identifier.
{{ if .AccessKey }}access_key: {{.AccessKey}}{{ else }}# access_key: SCW11111111111111111{{ end }}

# The secret key is the value that can be used to authenticate against the API (the value used in X-Auth-Token HTTP-header).
# The secret key MUST remain secret and not given to anyone or published online.
{{ if .SecretKey }}secret_key: {{ .SecretKey }}{{ else }}# secret_key: 11111111-1111-1111-1111-111111111111{{ end }}

# Your organization ID is the identifier of your account inside Scaleway infrastructure.
{{ if .DefaultOrganizationID }}default_organization_id: {{ .DefaultOrganizationID }}{{ else }}# default_organization_id: 11111111-1111-1111-1111-111111111111{{ end }}

# A region is represented as a geographical area such as France (Paris) or the Netherlands (Amsterdam).
# It can contain multiple availability zones.
# Example of region: fr-par, nl-ams
{{ if .DefaultRegion }}default_region: {{ .DefaultRegion }}{{ else }}# default_region: fr-par{{ end }}

# A region can be split into many availability zones (AZ).
# Latency between multiple AZ of the same region are low as they have a common network layer.
# Example of zones: fr-par-1, nl-ams-1
{{ if .DefaultZone }}default_zone: {{.DefaultZone}}{{ else }}# default_zone: fr-par-1{{ end }}

# APIURL overrides the API URL of the Scaleway API to the given URL.
# Change that if you want to direct requests to a different endpoint.
{{ if .APIURL }}apiurl: {{ .APIURL }}{{ else }}# api_url: https://api.scaleway.com{{ end }}

# Insecure enables insecure transport on the client.
# Default to false
{{ if .Insecure }}insecure: {{ .Insecure }}{{ else }}# insecure: false{{ end }}

# A configuration is a named set of Scaleway properties.
# Starting off with a Scaleway SDK or Scaleway CLI, you’ll work with a single configuration named default.
# You can set properties of the default profile by running either scw init or scw config set. 
# This single default configuration is suitable for most use cases.
{{ if .ActiveProfile }}active_profile: {{ .ActiveProfile }}{{ else }}# active_profile: myProfile{{ end }}

# To improve the Scaleway CLI we rely on diagnostic and usage data.
# Sending such data is optional and can be disable at any time by setting send_telemetry variable to false.
{{ if .SendTelemetry }}send_telemetry: {{ .SendTelemetry }}{{ else }}# send_telemetry: false{{ end }}

# To work with multiple projects or authorization accounts, you can set up multiple configurations with scw config configurations create and switch among them accordingly.
# You can use a profile by either:
# - Define the profile you want to use as the SCW_PROFILE environment variable
# - Use the GetActiveProfile() function in the SDK
# - Use the --profile flag with the CLI

# You can define a profile using the following syntax:
{{ if gt (len .Profiles) 0 }}
profiles:
{{- range $k,$v := .Profiles }}
  {{ $k }}:
    {{ if $v.AccessKey }}access_key: {{ $v.AccessKey }}{{ else }}# access_key: SCW11111111111111111{{ end }}
    {{ if $v.SecretKey }}secret_key: {{ $v.SecretKey }}{{ else }}# secret_key: 11111111-1111-1111-1111-111111111111{{ end }}
    {{ if $v.DefaultOrganizationID }}default_organization_id: {{ $v.DefaultOrganizationID }}{{ else }}# default_organization_id: 11111111-1111-1111-1111-111111111111{{ end }}
    {{ if $v.DefaultZone }}default_zone: {{ $v.DefaultZone }}{{ else }}# default_zone: fr-par-1{{ end }}
    {{ if $v.DefaultRegion }}default_region: {{ $v.DefaultRegion }}{{ else }}# default_region: fr-par{{ end }}
    {{ if $v.APIURL }}api_url: {{ $v.APIURL }}{{ else }}# api_url: https://api.scaleway.com{{ end }}
    {{ if $v.Insecure }}insecure: {{ $v.Insecure }}{{ else }}# insecure: false{{ end }}
{{ end }}
{{- else }}
# profiles:
#   myProfile:
#     access_key: 11111111-1111-1111-1111-111111111111
#     secret_key: 11111111-1111-1111-1111-111111111111
#     organization_id: 11111111-1111-1111-1111-111111111111
#     default_zone: fr-par-1
#     default_region: fr-par
#     api_url: https://api.scaleway.com
#     insecure: false
{{ end -}}
`

type Config struct {
	Profile       `yaml:",inline"`
	ActiveProfile *string             `yaml:"active_profile,omitempty"`
	SendTelemetry *bool               `yaml:"send_telemetry,omitempty"`
	Profiles      map[string]*Profile `yaml:"profiles,omitempty"`
}

type Profile struct {
	AccessKey             *string `yaml:"access_key,omitempty"`
	SecretKey             *string `yaml:"secret_key,omitempty"`
	APIURL                *string `yaml:"api_url,omitempty"`
	Insecure              *bool   `yaml:"insecure,omitempty"`
	DefaultOrganizationID *string `yaml:"default_organization_id,omitempty"`
	DefaultRegion         *string `yaml:"default_region,omitempty"`
	DefaultZone           *string `yaml:"default_zone,omitempty"`
}

func (p *Profile) String() string {
	p2 := *p
	p2.SecretKey = hideSecretKey(p2.SecretKey)
	configRaw, _ := yaml.Marshal(p2)
	return string(configRaw)
}

// clone deep copy config object
func (c *Config) clone() *Config {
	c2 := &Config{}
	configRaw, _ := yaml.Marshal(c)
	_ = yaml.Unmarshal(configRaw, c2)
	return c2
}

func (c *Config) String() string {
	c2 := c.clone()
	c2.SecretKey = hideSecretKey(c2.SecretKey)
	for _, p := range c2.Profiles {
		p.SecretKey = hideSecretKey(p.SecretKey)
	}

	configRaw, _ := yaml.Marshal(c2)
	return string(configRaw)
}

func (c *Config) IsEmpty() bool {
	return c.String() == "{}\n"
}

func hideSecretKey(key *string) *string {
	if key == nil {
		return nil
	}

	newKey := auth.HideSecretKey(*key)
	return &newKey
}

func unmarshalConfV2(content []byte) (*Config, error) {
	var config Config

	err := yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// MustLoadConfig is like LoadConfig but panic instead of returning an error.
func MustLoadConfig() *Config {
	c, err := LoadConfigFromPath(GetConfigPath())
	if err != nil {
		panic(err)
	}
	return c
}

// LoadConfig read the config from the default path.
func LoadConfig() (*Config, error) {
	return LoadConfigFromPath(GetConfigPath())
}

// LoadConfigFromPath read the config from the given path.
func LoadConfigFromPath(path string) (*Config, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		if pathError, isPathError := err.(*os.PathError); isPathError && pathError.Err == syscall.ENOENT {
			return nil, configFileNotFound(pathError.Path)
		}
		return nil, errors.Wrap(err, "cannot read config file")
	}

	_, err = unmarshalConfV1(file)
	if err == nil {
		// reject V1 config
		return nil, errors.New("found legacy config in %s: legacy config is not allowed, please switch to the new config file format: %s", path, documentationLink)
	}

	confV2, err := unmarshalConfV2(file)
	if err != nil {
		return nil, errors.Wrap(err, "content of config file %s is invalid", path)
	}

	return confV2, nil
}

// GetProfile returns the profile corresponding to the given profile name.
func (c *Config) GetProfile(profileName string) (*Profile, error) {
	if profileName == "" {
		return nil, errors.New("active profile cannot be empty")
	}

	p, exist := c.Profiles[profileName]
	if !exist {
		return nil, errors.New("given profile %s does not exist", profileName)
	}

	// Merge selected profile on top of default profile
	return MergeProfiles(&c.Profile, p), nil
}

// GetActiveProfile returns the active profile of the config based on the following order:
// env SCW_PROFILE > config active_profile > config root profile
func (c *Config) GetActiveProfile() (*Profile, error) {
	switch {
	case os.Getenv(scwActiveProfileEnv) != "":
		logger.Debugf("using active profile from env: %s=%s", scwActiveProfileEnv, os.Getenv(scwActiveProfileEnv))
		return c.GetProfile(os.Getenv(scwActiveProfileEnv))
	case c.ActiveProfile != nil:
		logger.Debugf("using active profile from config: active_profile=%s", scwActiveProfileEnv, *c.ActiveProfile)
		return c.GetProfile(*c.ActiveProfile)
	default:
		return &c.Profile, nil
	}
}

// SaveTo will save the config to the default config path. This
// action will overwrite the previous file when it exists.
func (c *Config) Save() error {
	return c.SaveTo(GetConfigPath())
}

// HumanConfig will generate a config file with documented arguments.
func (c *Config) HumanConfig() (string, error) {
	tmpl, err := template.New("configuration").Parse(configFileTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, c)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// SaveTo will save the config to the given path. This action will
// overwrite the previous file when it exists.
func (c *Config) SaveTo(path string) error {
	path = filepath.Clean(path)

	// STEP 1: Render the configuration file as a file
	file, err := c.HumanConfig()
	if err != nil {
		return err
	}

	// STEP 2: create config path dir in cases it didn't exist before
	err = os.MkdirAll(filepath.Dir(path), 0700)
	if err != nil {
		return err
	}

	// STEP 3: write new config file
	err = ioutil.WriteFile(path, []byte(file), defaultConfigPermission)
	if err != nil {
		return err
	}

	return nil
}

// MergeProfiles merges profiles in a new one. The last profile has priority.
func MergeProfiles(original *Profile, others ...*Profile) *Profile {
	np := &Profile{
		AccessKey:             original.AccessKey,
		SecretKey:             original.SecretKey,
		APIURL:                original.APIURL,
		Insecure:              original.Insecure,
		DefaultOrganizationID: original.DefaultOrganizationID,
		DefaultRegion:         original.DefaultRegion,
		DefaultZone:           original.DefaultZone,
	}

	for _, other := range others {
		if other.AccessKey != nil {
			np.AccessKey = other.AccessKey
		}
		if other.SecretKey != nil {
			np.SecretKey = other.SecretKey
		}
		if other.APIURL != nil {
			np.APIURL = other.APIURL
		}
		if other.Insecure != nil {
			np.Insecure = other.Insecure
		}
		if other.DefaultOrganizationID != nil {
			np.DefaultOrganizationID = other.DefaultOrganizationID
		}
		if other.DefaultRegion != nil {
			np.DefaultRegion = other.DefaultRegion
		}
		if other.DefaultZone != nil {
			np.DefaultZone = other.DefaultZone
		}
	}

	return np
}
