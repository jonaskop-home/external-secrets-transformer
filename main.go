package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type SecretStoreRef struct {
	Name string
	Kind string
}

type Template struct {
	Data map[string]string
}

type Target struct {
	Template Template
}

type RemoteRef struct {
	Key string
}

type SingleData struct {
	SecretKey string    `yaml:"secretKey"`
	RemoteRef RemoteRef `yaml:"remoteRef"`
}

type ExternalSecretSpec struct {
	RefreshInterval string         `yaml:"refreshInterval"`
	SecretStoreRef  SecretStoreRef `yaml:"secretStoreRef"`
	Target          Target
	Data            []SingleData
}

func getEnvPanic(name string) string {
	value := os.Getenv(name)
	if len(value) == 0 {
		panic("ERROR: Environment variable '$" + name + "'is not set")
	}
	return value
}

func getEnvDefault(name, defaultValue string) string {
	value := os.Getenv(name)
	if len(value) > 0 {
		return value
	}
	return defaultValue
}

func createBasicExternalSecretSpec() ExternalSecretSpec {
	return ExternalSecretSpec{
		RefreshInterval: getEnvDefault("REFRESH_INTERVAL", "1h"),
		SecretStoreRef: SecretStoreRef{
			Name: getEnvPanic("STORE_NAME"),
			Kind: getEnvPanic("STORE_KIND"),
		},
		Target: Target{Template: Template{
			Data: make(map[string]string),
		}},
		Data: make([]SingleData, 0),
	}
}

func parseStringDataAndData(secretManifest map[string]interface{}) map[string]interface{} {
	data := make(map[string]interface{})
	if secretManifest["stringData"] != nil {
		data = secretManifest["stringData"].(map[string]interface{})
	}
	if secretManifest["data"] != nil {
		for k, v := range secretManifest["data"].(map[string]interface{}) {
			decodedBytes, err := base64.StdEncoding.DecodeString(v.(string))
			if err != nil {
				errors.Wrap(err, "Could not decode base64 string")
				return data
			}
			data[k] = string(decodedBytes)
		}
	}
	return data
}

func getKeyvaultVariables(data map[string]interface{}) []string {
	var keyvaultKeys []string

	for _, v := range data {
		r := regexp.MustCompile(`\{{([^}]+)\}}`)
		matches := r.FindAllString(v.(string), -1)
		for _, match := range matches {
			formatted := match[2 : len(match)-2]
			words := strings.Fields(formatted)
			for _, w := range words {
				if w[0] == '.' {
					keyvaultKeys = append(keyvaultKeys, w)
					break
				}
			}
		}

	}
	return keyvaultKeys
}

func createExternalSecretObject(data, doc map[string]interface{}, keyvaultKeys []string) map[string]interface{} {
	// Create external secret spec
	spec := createBasicExternalSecretSpec()

	for k, v := range data {
		spec.Target.Template.Data[k] = v.(string)
	}

	for _, key := range keyvaultKeys {
		if !strings.HasPrefix(key, ".") {
			panic("ERROR: Key is missing '.' prefix '" + key + "'")
		}
		spec.Data = append(spec.Data, SingleData{
			SecretKey: key[1:],
			RemoteRef: RemoteRef{Key: key[1:]},
		})
	}
	doc["kind"] = "ExternalSecret"
	doc["apiVersion"] = "external-secrets.io/v1beta1"
	delete(doc, "data")
	delete(doc, "stringData")
	doc["spec"] = spec
	delete(doc, "type")

	return doc
}

func main() {

	bytes12, _ := io.ReadAll(os.Stdin)

	dec := yaml.NewDecoder(bytes.NewReader(bytes12))

	for {
		var doc map[string]interface{}
		if dec.Decode(&doc) != nil {
			break
		}
		if doc == nil {
			continue
		}
		kind := doc["kind"].(string)
		apiVersion := doc["apiVersion"].(string)
		if kind == "Secret" && apiVersion == "v1" {
			// Convert data from base64 and merge stringData with data
			data := parseStringDataAndData(doc)

			// Find all keyvault variables
			keyvaultKeys := getKeyvaultVariables(data)

			if len(keyvaultKeys) > 0 {
				doc = createExternalSecretObject(data, doc, keyvaultKeys)
			}

		}
		var b bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&b)
		yamlEncoder.SetIndent(2)
		yamlEncoder.Encode(&doc)
		fmt.Println(string(b.Bytes()))
		fmt.Println("---")
	}
}
