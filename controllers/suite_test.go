package rollouts

import (
	"flag"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), zap.Level(zapcore.DebugLevel)))
})

func TestRollouts(t *testing.T) {

	suiteConfig, _ := GinkgoConfiguration()

	// Define a flag for the poll progress after interval
	var pollProgressAfter time.Duration
	// A test is "slow" if it takes longer than a few minutes
	flag.DurationVar(&pollProgressAfter, "poll-progress-after", 3*time.Minute, "Interval for polling progress after")

	// Parse the flags
	flag.Parse()

	// Set the poll progress after interval in the suite configuration
	suiteConfig.PollProgressAfter = pollProgressAfter

	RegisterFailHandler(Fail)
	RunSpecs(t, "Rollouts Suite")
}
