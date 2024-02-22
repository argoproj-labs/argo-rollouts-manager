package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

func main() {

	// As of February 2024 (Rollouts v1.6.6):
	//
	// Always fail:
	// - TestAPISIXCanarySetHeaderStep
	// - TestExperimentWithDryRunMetrics (also fails when running upstream rollouts as a container)
	// - TestControllerMetrics (also fails when running upstream rollouts as a container)
	//
	// Intermittently fail:
	// - TestCanaryDynamicStableScale
	// - TestCanaryScaleDownOnAbort
	// - TestBlueGreenPromoteFull
	// - TestALBExperimentStepNoSetWeight
	// - TestALBExperimentStep
	// - TestALBExperimentStepNoSetWeightMultiIngress

	// - Most of these tests fail 100% of the time, because they were not designed to run against Argo Rollouts running on a cluster. (These are safe to ignore.)
	//   - The rollouts tests are written to assume they are running locally: not within a container, and not on a K8s cluster.
	// - Some are intermittently failing, implying a race condition in the test/product.

	testsExpectedToFailList := []string{
		"TestAPISIXSuite/TestAPISIXCanarySetHeaderStep",
		"TestExperimentSuite/TestExperimentWithDryRunMetrics",
		"TestFunctionalSuite/TestControllerMetrics",
	}

	// DONE 6 runs, 144 tests, 6 skipped, 47 failures in 2279.668s

	// 1) Read E2E test log, output by run-upstream-argo-rollouts-e2e-tests.sh
	if len(os.Args) != 2 {
		reportErrorAndExit(fmt.Errorf("expected args: path to E2E test log"))
		return
	}

	testsE2EResultsLogPath := os.Args[1]

	fileContents, err := waitAndGetE2EFileContents(testsE2EResultsLogPath)
	if err != nil {
		reportErrorAndExit(err)
		return
	}
	fmt.Println()

	// 2) Parse E2E test log, skipping any tests that we expect to fail
	testsExpectedToFailMap := map[string]any{}

	for _, testExpectedToFail := range testsExpectedToFailList {
		testsExpectedToFailMap[testExpectedToFail] = ""
	}

	// map: name of failed test -> list of failed test runs
	testResults := map[string][]string{}

	for _, line := range fileContents {

		failPrefix := "=== FAIL: test/e2e"

		var testName string

		if strings.HasPrefix(line, failPrefix) {
			// example: === FAIL: test/e2e TestFunctionalSuite/TestBlueGreenPromoteFull (unknown)

			testName = line[len(failPrefix)+1 : strings.Index(line, "(")-1]

			if !strings.Contains(testName, "/") {
				// Ignore the fail reported for the suite: we only care about individual test fails
				continue
			}

		} else if strings.HasPrefix(line, "PASS") {
			// example: PASS test/e2e.TestCanarySuite/TestCanaryDynamicStableScale (20.91s)

			roundBraceIndex := strings.Index(line, "(")

			if roundBraceIndex == -1 {
				continue
			}

			testName = line[strings.Index(line, ".")+1 : roundBraceIndex-1]

		} else {
			continue
		}

		if _, exists := testsExpectedToFailMap[testName]; exists {
			// Ignore tests that are expected to fail
			continue
		}

		if !strings.Contains(testName, "/") {
			// Skip suite-only results
			continue
		}

		testResults[testName] = append(testResults[testName], line)
	}

	// 3) Report unexpected failed tests, in alphabetical order
	mapKeys := []string{}
	for key := range testResults {
		mapKeys = append(mapKeys, key)
	}

	slices.Sort(mapKeys)

	atLeastOneTestFailure := false

	for _, testName := range mapKeys {

		testRuns := testResults[testName]

		// gotestsum will rerun Argo Rollouts tests up to 5 times (6 runs total)
		// - Here we check if there exists AT LEAST ONE 'PASS'
		// - If no 'PASS' results exist, it implies the test never passed after all retries, so it's a permfail

		passFound := false
		for _, testRunLine := range testRuns {
			if strings.Contains(testRunLine, "PASS") {
				passFound = true
			}
		}

		if !passFound {
			fmt.Println("Unexpected test failure, test failed too many times - " + testName + ":")
			for _, testRun := range testRuns {
				fmt.Println(testRun)
			}

			fmt.Println()
			atLeastOneTestFailure = true

		}

	}

	// 4) Exit with error code 1 if there was at least one unexpected test failure.
	if atLeastOneTestFailure {
		reportErrorAndExit(fmt.Errorf("at least one test failure occurred"))
		return
	}

}

// waitAndGetE2EFileContents waits for the last line of the file to start with 'DONE' before returning the contents
// This allows us to avoid a race condition with tee where the file contents of the file may not have been fully flushed.
func waitAndGetE2EFileContents(testsE2EResultsLogPath string) ([]string, error) {

	for {

		fileLines, err := readFileIntoListOfLines(testsE2EResultsLogPath, true)
		if err != nil {
			return []string{}, err
		}

		lastLine := fileLines[len(fileLines)-1]

		// example: DONE 6 runs, 144 tests, 6 skipped, 47 failures in 2279.668s
		if strings.HasPrefix(lastLine, "DONE ") {
			fmt.Println("E2E tests file is complete:", lastLine)
			return fileLines, nil
		}

		// Otherwise, wait a second then check again.
		fmt.Println("* Waiting for E2E test file to be complete:", lastLine)
		time.Sleep(time.Second)
	}

}

func reportErrorAndExit(err error) {
	fmt.Println("ERROR:", err)
	os.Exit(1)
}

func readFileIntoListOfLines(path string, initialTrimSpace bool) ([]string, error) {
	fileContentsBytes, err := os.ReadFile(path)
	if err != nil {
		return []string{}, err
	}

	fileContents := string(fileContentsBytes)

	if initialTrimSpace {
		fileContents = strings.TrimSpace(fileContents)
	}

	return strings.Split(string(fileContents), "\n"), nil

}
