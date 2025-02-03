package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {

	// As of March 2024 (Rollouts v1.6.6):
	//
	// Always fail:
	// - TestAPISIXCanarySetHeaderStep
	// - TestExperimentWithDryRunMetrics (also fails when running upstream rollouts as a container)
	// - TestControllerMetrics (also fails when running upstream rollouts as a container)
	//
	// Often fail:
	// - TestBlueGreenPromoteFull
	//
	// Intermittently fail:
	// - TestCanaryDynamicStableScale
	// - TestCanaryScaleDownOnAbort
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
		"TestFunctionalSuite/TestBlueGreenPromoteFull",
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
	// - map: name of test -> list of test run lines
	testResults, err := parseTestResultsFromFile(fileContents, testsExpectedToFailList)
	if err != nil {
		reportErrorAndExit(err)
		return
	}

	// 3a) Report unexpected failed tests, in alphabetical order
	mapKeys := []string{}
	for key := range testResults {
		mapKeys = append(mapKeys, key)
	}

	sort.Strings(mapKeys)

	if len(mapKeys) == 0 { // sanity test
		reportErrorAndExit(fmt.Errorf("no test results found"))
		return
	}

	var testFailureOutput []string
	var testSuccessOutput []string

	atLeastOneTestPermFail := false

	for _, testName := range mapKeys {

		testRuns := testResults[testName]

		// gotestsum will rerun Argo Rollouts tests up to 5 times (6 runs total)
		// - Here we check if there exists AT LEAST ONE 'PASS'
		// - If no 'PASS' results exist, it implies the test never passed after all retries, so it's a permfail

		passFound := false
		for _, testRunLine := range testRuns {
			if strings.Contains(testRunLine, "PASS") {
				passFound = true
				testSuccessOutput = append(testSuccessOutput, "Test passed - "+testName)
				break
			}
		}

		if !passFound {

			testFailureOutput = append(testFailureOutput, "Unexpected test failure, test failed too many times - "+testName+":")

			testFailureOutput = append(testFailureOutput, testRuns...)

			testFailureOutput = append(testFailureOutput, "")
			atLeastOneTestPermFail = true

		}

	}

	fmt.Println()

	// 3b) First print successes
	for _, line := range testSuccessOutput {
		fmt.Println(line)
	}

	fmt.Println()

	// 3c) Then print failures
	for _, line := range testFailureOutput {
		fmt.Println(line)
	}

	// 4) Exit with error code 1 if there was at least one unexpected test failure.
	if atLeastOneTestPermFail {
		reportErrorAndExit(fmt.Errorf("at least one test failure occurred"))
		return
	}

}

// parseTestResultsFromFile parses the E2E test log, skipping any tests that we expect to fail, and storing the results in a map
func parseTestResultsFromFile(fileContents []string, testsExpectedToFailList []string) (map[string][]string, error) {

	atLeastOnePassSeen := false
	atLeastOneFailSeen := false

	testResults := map[string][]string{}

	testsExpectedToFailMap := map[string]any{}

	for _, testExpectedToFail := range testsExpectedToFailList {
		testsExpectedToFailMap[testExpectedToFail] = ""
	}

	for _, line := range fileContents {

		testSlashE2EText := "test/e2e"

		if !strings.Contains(line, testSlashE2EText) {
			continue
		}

		var testName string

		if strings.HasPrefix(line, "===") && strings.Contains(line, "FAIL") {
			// example: === FAIL: test/e2e TestFunctionalSuite/TestBlueGreenPromoteFull (unknown)
			//
			// Unfortunately, the FAIL line is surrounded by invisible ANSI colour whitespace, so we can't scan for it directly.
			// - we instead check for ===, FAIL, and the test/e2e string.

			testName = line[strings.Index(line, testSlashE2EText)+len(testSlashE2EText)+1 : strings.Index(line, "(")-1]
			testName = strings.TrimSpace(testName)

			if !strings.Contains(testName, "/") {
				// Ignore the fail reported for the suite: we only care about individual test fails
				continue
			}

			atLeastOneFailSeen = true

		} else if strings.Contains(line, "PASS") && strings.Contains(line, ".") {
			// example: PASS test/e2e.TestCanarySuite/TestCanaryDynamicStableScale (20.91s)

			roundBraceIndex := strings.Index(line, "(")

			if roundBraceIndex == -1 {
				continue
			}

			dotIndex := strings.Index(line, ".")
			if dotIndex == -1 {
				continue
			}

			if dotIndex+1 >= roundBraceIndex-1 {
				fmt.Println("Unexpected line format: [", line, "]")
				continue
			}

			testName = line[dotIndex+1 : roundBraceIndex-1]
			atLeastOnePassSeen = true

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

	if !atLeastOneFailSeen || !atLeastOnePassSeen { // sanity test: BOTH a pass and fail should have occurred in the log.
		return nil, fmt.Errorf("there may be something wrong with the parser: we expect to see both at least one pass, and at least one fail, in the parsed output: %v %v", atLeastOneFailSeen, atLeastOnePassSeen)
	}

	return testResults, nil
}

// waitAndGetE2EFileContents waits for one of the last 50 lines of the file to start with 'DONE' before returning the contents.
// This allows us to avoid a race condition with tee where the file contents of the file may not have been fully written.
func waitAndGetE2EFileContents(testsE2EResultsLogPath string) ([]string, error) {

	// Return whatever we have after 1 minute has elapsed
	expireTime := time.Now().Add(1 * time.Minute)

	for {

		fileLines, err := readFileIntoListOfLines(testsE2EResultsLogPath, true)
		if err != nil {
			return []string{}, err
		}

		lineIndexStart := len(fileLines) - 50
		if lineIndexStart < 0 {
			lineIndexStart = 0
		}

		for _, line := range fileLines[lineIndexStart:] {
			// example line: DONE 6 runs, 144 tests, 6 skipped, 47 failures in 2279.668s
			if strings.HasPrefix(line, "DONE ") {
				fmt.Println("E2E tests file is complete:", line)
				return fileLines, nil
			}
		}

		if time.Now().After(expireTime) {
			fmt.Println("E2E tests file timed out waiting. Previous X lines were:", fileLines[lineIndexStart:])
			return fileLines, nil
		}

		// Otherwise, wait a second then check again.
		fmt.Println("* Waiting for E2E test file to be complete")
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
