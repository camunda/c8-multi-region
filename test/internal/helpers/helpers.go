package helpers

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
)

// Struct Helper

type Cluster struct {
	Region           string
	ClusterName      string
	KubectlNamespace k8s.KubectlOptions
	KubectlSystem    k8s.KubectlOptions
	KubectlFailover  k8s.KubectlOptions
}

// Go Helpers
func GetEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

func CutOutString(originalString, searchString string) int {
	re := regexp.MustCompile(searchString)
	matches := re.FindStringSubmatch(originalString)

	if (len(matches)) == 0 {
		return -1
	}

	length := len(matches[0])
	if (length) == 0 {
		return -1
	}

	num, err := strconv.Atoi(string(matches[0][length-1]))
	if err != nil {
		return -1
	}

	return num
}

func IsEven(num int) bool {
	if num < 0 {
		return false
	}

	return num%2 == 0
}

func IsOdd(num int) bool {
	if num < 0 {
		return false
	}

	return num%2 == 1
}

// Terraform Helpers
func FetchSensitiveTerraformOutput(t *testing.T, options *terraform.Options, name string) string {
	defer func() {
		options.Logger = nil
	}()
	options.Logger = logger.Discard
	return terraform.Output(t, options, name)
}

// Combine two maps into a single map of string key-value pairs
func CombineMaps(map1, map2 map[string]string) map[string]string {
	// Create a new map to hold the combined result
	combined := make(map[string]string)

	// Copy all key-value pairs from map1 to combined map
	for key, value := range map1 {
		combined[key] = value
	}

	// Iterate through map2 and insert key-value pairs into combined map
	for key, value := range map2 {
		// If the key already exists in map1, overwrite the value of the existing key
		combined[key] = value
	}

	return combined
}

// Overwrite the image tag for Camunda images in the map with the provided tag
func OverwriteImageTag(map1 map[string]string, tag string) map[string]string {
	// Allows to later add additional images to the map like Optimize / Connectors
	map1["zeebe.image.tag"] = tag
	map1["zeebeGateway.image.tag"] = tag
	map1["operate.image.tag"] = tag
	map1["tasklist.image.tag"] = tag

	return map1
}
