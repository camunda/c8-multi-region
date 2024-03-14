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
