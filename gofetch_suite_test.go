package gofetch_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGofetch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gofetch Suite")
}
