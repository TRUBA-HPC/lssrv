/*
 * List free servers - Show state of partitions managed by slurm.
 * Copyright (C) 2024  Hakan Bayindir
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

/*
 * This structure stores all of our runtime configuration which will be used during the
 * run. It'll be initialized with the default values first, and if we have a config file,
 * that config file will override these options.
 */
type RuntimeConfiguration struct {
	configurationFilePath string   // Full path of the configuration file in use.
	tableTitle            string   // The string shown in the title of the table.
	ignoredPartitions     []string // List of partitions that won't be shown on the table.
	queueStateFilePath    string   // Full path of the state queue file.
	softwareVersion       string   // Version of our software.
}

// This is our basic data structure which holds everything about a SLURM partition.
type Partition struct {
	// Basic information for this partition.
	name  string //Name of the partition
	state string // Normally a partition can be up or down. So it may change to bool later.

	// Following are the stats about our processors.
	totalProcessors     string // Total number of processors in this partition.
	allocatedProcessors string // Number of allocated processors in this partition.
	idleProcessors      string // Number of idle processors in this partition.
	otherProcessors     string // Number of processors in "other" state in this partition.

	// Following are related about nodes in the partition.
	totalNodes string // Total number of nodes (servers) in this partition.

	// Processor geometry per node in this partition.
	socketsPerNode    string // How many CPU sockets a node have.
	coresPerSocket    string // How many cores we have per CPU socket.
	threadsPerCore    string // How many threads a core can execute (It's 2 for HyperThreading/SMT systems).
	totalCoresPerNode string // How many cores we have in total, per node.

	// Nodes' memory is also a concern for us.
	totalMemoryPerNode       string // This is RAM per node, in megabytes.
	totalMemoryPerCore       uint   // This contains RAM per core, in megabytes.
	totalMemoryPerCoreSuffix string // If the partition is heterogenous, sinfo reports the minimum amount, followed by a "+". This is where we store it.

	// Job parameters for this partition.
	minimumNodesPerJob string // Minimum number of nodes we can allocate per job.
	maximumNodesPerJob string // Maximum number of nodes we can allocate per job.
	maximumCoresPerJob string // Maximum number of cores allowed per job.

	// Time limits imposed by this partition.
	// Left as string because we don't plan to process them at this point.
	defaultTimePerJob string // Default time if no time constraint is given in the job file.
	maximumTimePerJob string // The hard-coded upper time limit per job.

	// Stats about jobs on this partition
	totalJobsCount           uint // Total number of jobs on this partition.
	runningJobsCount         uint // Number of running jobs on this partition.
	waitingJobsCount         uint // Number of waiting jobs on this partition. Doesn't contain resource waiting jobs.
	resourceWaitingJobsCount uint // Total number of jobs waiting because of resources.
}

/*
 * This function fills our data structure with the initial values, so the software can
 * run without any configuration file, if necessary.
 */
func initializeDefeaultValues(runtimeConfiguration *RuntimeConfiguration, logger *zap.SugaredLogger) {
	// Won't be touching configurationFilePath and ignoredPartitions, because they are empty by default.

	// Use a generic name, so the software stays reusable.
	runtimeConfiguration.tableTitle = "Slurm partitions state"
	logger.Debugf("Table title is set to %s.", runtimeConfiguration.tableTitle)

	// We expect this to be installed system-wide.
	runtimeConfiguration.queueStateFilePath = "/var/cache/lssrv/queue.state"
	logger.Debugf("Queue state file path is %s.", runtimeConfiguration.queueStateFilePath)

	// Lastly, set the version.
	runtimeConfiguration.softwareVersion = "2.0.0"
	logger.Debugf("Software internal version is %s.", runtimeConfiguration.softwareVersion)
}

/*
 * This function finds the configuration file (if there's one), parses it and applies the
 * configuration to our runtime configuration. The system-wide paths are specially
 * prioritized because this tool is designed to be a system-wide one.
 */
func readAndApplyConfiguration(runtimeConfiguration *RuntimeConfiguration, logger *zap.SugaredLogger) {
	// Let's define the nature of our configuration file.
	viper.SetConfigName("lssrv.conf")
	viper.SetConfigType("toml")

	// We need user's config directory to add some neat magic.
	userConfigDirectory, err := os.UserConfigDir()

	if err != nil {
		logger.Panicf("Error getting user's config directory, exiting (error is %s).", err)
	}

	workingDirectory, err := os.Getwd()
	logger.Debugf("Current directory is %s", workingDirectory)

	if err != nil {
		logger.Panicf("Error getting working directory, exiting (error is %s).", err)
	}

	// The logic is to allow user to override local config with a test config.
	// And to override system-wide config with a local config.
	viper.AddConfigPath("/etc")              // A system-wide config should be present.
	viper.AddConfigPath(userConfigDirectory) // User might create a user-wide config for testing.
	viper.AddConfigPath(workingDirectory)    // This is a last resort and another test directory.

	err = viper.ReadInConfig() // Find and read the config file

	// Handle errors reading the config file.
	// Do not create warnings here, all config warnings are handled by checkConfigurationSanity() function.
	if err != nil {
		logger.Debugf("Cannot find any configuration file, continuing using built-in defaults (%s).", err)
	} else {
		// If we've reached here, we can take note of the file path now.
		// This also means that we have read the config file successfully.
		runtimeConfiguration.configurationFilePath = viper.ConfigFileUsed()

		// Let the user know where the config file is.
		logger.Debugf("Configuration file is found at %s.", runtimeConfiguration.configurationFilePath)

		// Check options one by one whether they're set and apply present options.
		// We do not panic if anything mandatory is missing, but we'll warn the user politely.
		// Below is the pushover section.
		if viper.IsSet("general.table_title") {
			runtimeConfiguration.tableTitle = viper.GetString("general.table_title")
			logger.Debugf("Table title is found & set to %s.", runtimeConfiguration.tableTitle)
		}

		if viper.IsSet("general.ignored_partitions") {
			runtimeConfiguration.ignoredPartitions = viper.GetStringSlice("general.ignored_partitions")
			logger.Debugf("%d ignored partition(s) found & set to %s.", len(runtimeConfiguration.ignoredPartitions), runtimeConfiguration.ignoredPartitions)
		}

		if viper.IsSet("general.queue_state_file_path") {
			runtimeConfiguration.queueStateFilePath = viper.GetString("general.queue_state_file_path")
			logger.Debugf("Queue state file path is found & set to %s.", runtimeConfiguration.queueStateFilePath)
		}
	}
}

/*
 * This function runs the sinfo command with a hard coded argument list.
 * The output we want is well defined, so it can be parsed with ease.
 */
func getPartitionsInformation(logger *zap.SugaredLogger) []byte {
	// Check whether we have the command in the OS, so we can continue safely.
	path, err := exec.LookPath("sinfo")

	// And exit gracefully if we don't have that.
	if err != nil {
		logger.Fatalf("Cannot find sinfo executable (error is %s), exiting.", err)
		os.Exit(1)
	}

	logger.Debugf("sinfo is found at %s.", path)

	// Build a command object so we can run it.
	commandToRun := exec.Command(path, "--format=%R|%a|%D|%B|%c|%C|%s|%z|%l|%m")

	// Let it run!
	logger.Debugf("Will be running the command %s with args %s.", commandToRun.Path, commandToRun.Args)
	partitionsInformation, err := commandToRun.Output()

	if err != nil {
		logger.Fatalf("%s command returned an error (error is %s)", path, err)
	}

	// We have an extra line at the end of the output, let's trim this.
	partitionsInformation = bytes.TrimRight(partitionsInformation, "\n")

	logger.Debugf("Got the partitions' information, returning.")
	return partitionsInformation
}

/*
 * This function parses the partition information we got from sinfo command and returns us
 * the partition map we'll further process with waiting and total jobs data.
 */
func parsePartitionsInformation(partitionsInformation []byte, logger *zap.SugaredLogger) map[string]Partition {
	// We'll start by dividing the partition information to lines.
	partitionsInformationLines := bytes.Split(partitionsInformation, []byte("\n"))

	// Let's see what we got.
	logger.Debugf("Partition information has returned %d line(s).", len(partitionsInformationLines))

	/*
	 * The idea here is to compare the header with a known good value to check that
	 * everything is formatted the way we want. Then we can continue parsing the rest.
	 */
	logger.Debugf("Checking command output, comparing headers.")

	referenceHeader := "PARTITION|AVAIL|NODES|MAX_CPUS_PER_NODE|CPUS|CPUS(A/I/O/T)|JOB_SIZE|S:C:T|TIMELIMIT|MEMORY"

	if string(partitionsInformationLines[0]) != referenceHeader {
		logger.Fatalf("Command output header doesn't match required template. This version of sinfo is not compatible, exiting.")
	}

	logger.Debugf("Header check passed.")

	// Discard the header since we don't need that anymore.
	partitionsInformationLines = partitionsInformationLines[1:]
	logger.Debugf("Partition information now has %d line(s).", len(partitionsInformationLines))

	// Create a map to hold the partitions, indexed by their name (hold that thought).
	partitionsMap := make(map[string]Partition)

	// Start traversing the partitions we have.
	for lineNumber, line := range partitionsInformationLines {
		logger.Debugf("Partition #%d is %s.", lineNumber, line)

		// And split them according to the divider we selected before.
		partitionFields := bytes.Split(line, []byte("|"))
		// Always show your results to check them.
		logger.Debugf("Partition line is splitted as %s.", partitionFields)

		// Let's create a partition and start to fill it up.
		var partitionToParse Partition

		partitionToParse.name = string(partitionFields[0])
		partitionToParse.state = string(partitionFields[1])

		// We'll be copying the string as is to the field, since we won't be doing anything with these as numbers.
		partitionToParse.totalNodes = string(partitionFields[2])
		logger.Debugf("Total node(s) in this partition is %s.", partitionToParse.totalNodes)

		// Maximum cores per job, for this partition.
		partitionToParse.maximumCoresPerJob = string(partitionFields[3])
		logger.Debugf("Maximum core(s) per job for this partition is %s.", partitionToParse.maximumCoresPerJob)

		// Next is CPUs per node.
		partitionToParse.totalCoresPerNode = string(partitionFields[4])
		logger.Debugf("Total core(s) per node is %s.", partitionToParse.totalCoresPerNode)

		// Now we will parse the processor counts per partition. It's stored as "Allocated/Idle/Other/Total".
		processorCounts := bytes.Split(partitionFields[5], []byte("/"))
		partitionToParse.allocatedProcessors = string(processorCounts[0])
		partitionToParse.idleProcessors = string(processorCounts[1])
		partitionToParse.otherProcessors = string(processorCounts[2])
		partitionToParse.totalProcessors = string(processorCounts[3])
		logger.Debugf("Processor counts for partition is %s/%s/%s/%s (Available/Idle/Other/Total).", partitionToParse.allocatedProcessors, partitionToParse.idleProcessors, partitionToParse.otherProcessors, partitionToParse.totalProcessors)

		// Next we'll handle the node count limitations per job. Format is "minimum-maximum"
		nodeLimits := bytes.Split(partitionFields[6], []byte("-"))
		partitionToParse.minimumNodesPerJob = string(nodeLimits[0])
		partitionToParse.maximumNodesPerJob = string(nodeLimits[1])
		logger.Debugf("Node count limits for this partition is between %s and %s.", partitionToParse.minimumNodesPerJob, partitionToParse.maximumNodesPerJob)

		// We have processor geometry per node, which again needs some processing. The format is "Sockets per node:Cores per socket:Threads per core".
		processorGeometry := bytes.Split(partitionFields[7], []byte(":"))
		partitionToParse.socketsPerNode = string(processorGeometry[0])
		partitionToParse.coresPerSocket = string(processorGeometry[1])
		partitionToParse.threadsPerCore = string(processorGeometry[2])
		logger.Debugf("Processor geometry for this partition is %s:%s:%s (Sockets per node:Cores per socket:Threads per core)", partitionToParse.socketsPerNode, partitionToParse.coresPerSocket, partitionToParse.threadsPerCore)

		// Time limit comes next.
		partitionToParse.maximumTimePerJob = string(partitionFields[8])
		logger.Debugf("Maximum time per job in this partition is %s.", partitionToParse.maximumTimePerJob)

		// Lastly we have the total memory amount per server.
		partitionToParse.totalMemoryPerNode = string(partitionFields[9])
		logger.Debugf("Total memory per node is %s.", partitionToParse.totalMemoryPerNode)

		// We will calculate memory per core. This will be a little involved.
		memoryInformation := bytes.Split(partitionFields[9], []byte("+"))
		logger.Debugf("Memory information contains %d part(s).", len(memoryInformation))

		// If we have the suffix, add it.
		if len(memoryInformation) > 1 {
			partitionToParse.totalMemoryPerCoreSuffix = "+"
		}

		// This part is a bit involved. We need to divide some numbers to get an actual value.
		// First, parse the number as a 64bit unsigned integer.
		totalMemoryPerNode, err := strconv.ParseUint(string(memoryInformation[0]), 10, 64)

		if err != nil {
			logger.Fatalf("Cannot convert total memory amount to uint (error is %s).", err)
		}

		// Then do the same parsing for total core count for the node.
		// We also need to handle the "+" suffix if the partition we're working on is heterogenous.
		totalCoreCountPerNode, err := strconv.ParseUint(string(bytes.Split(partitionFields[4], []byte("+"))[0]), 10, 64)

		if err != nil {
			logger.Fatalf("Cannot convert total core count to uint (error is %s).", err)
		}

		// Lastly, do the division and store the number.
		partitionToParse.totalMemoryPerCore = uint(totalMemoryPerNode) / uint(totalCoreCountPerNode)
		logger.Debugf("This partition has %d%sMB of RAM per core.", partitionToParse.totalMemoryPerCore, partitionToParse.totalMemoryPerCoreSuffix)

		// Add the completed partition to the map.
		partitionsMap[partitionToParse.name] = partitionToParse
	}
	return partitionsMap
}

/*
 * Following function parses the queue state function and adds the information to the
 * relevant partition. It basically parses the file line by line and counts the job
 * states.
 */
func parseQueueStateFile(runtimeConfig *RuntimeConfiguration, partitionsToUpdate *map[string]Partition, logger *zap.SugaredLogger) os.FileInfo {
	// As a good, defensive programmer, we need to make sure that the file is there and we can read it before trying to parse it.
	fileInfo, err := os.Stat(runtimeConfig.queueStateFilePath)

	if err != nil {
		logger.Fatalf("Cannot stat queue state file %s (error is %s).", runtimeConfig.queueStateFilePath, err)
	}

	// Let's print what we have found so far.
	logger.Debugf("File information is as follows:")
	logger.Debugf("File name: %s", fileInfo.Name())
	logger.Debugf("Is a directory: %t", fileInfo.IsDir())
	logger.Debugf("Permissions are: %s", fileInfo.Mode().Perm().String())
	logger.Debugf("Last modification time: %s", fileInfo.ModTime())

	// If we're here, it means we're good so far. Let's open the file in read-only mode.
	// Open is a shorthand for opening a file in read-only mode.
	stateFile, err := os.Open(runtimeConfig.queueStateFilePath)
	defer stateFile.Close() // Do not forget to close the file when the function ends.

	// Be vigilant!
	if err != nil {
		// TODO: Update the error message with a friendlier, more useful one.
		logger.Fatalf("There was an error while opening queue state file %s (error is %s).", runtimeConfig.queueStateFilePath, err)
	}

	// Read the file into the memory.
	// I'll be doing a complete read into memory, since the file is <50KB, and I don't have time to optimize this.
	// TODO: Convert this to buffered read in the future.
	queueStateToParse := make([]byte, fileInfo.Size())
	bytesRead, err := stateFile.Read(queueStateToParse)

	if err != nil {
		logger.Fatalf("There was an error reading the queue state file %s (error is %s).", runtimeConfig.queueStateFilePath, err)
	}

	logger.Debugf("Read %d byte(s) from the file %s.", bytesRead, runtimeConfig.queueStateFilePath)

	// Command leaves a lone newline at the end, let's clean it.
	queueStateToParse = bytes.TrimRight(queueStateToParse, "\n")

	// Let's divide the file to lines.
	queueStateToParseLines := bytes.Split(queueStateToParse, []byte("\n"))

	// Next, we'll do a header check, and will continue parsing the file.
	referenceHeader := "PARTITION|STATE|REASON"

	if string(bytes.TrimRight(queueStateToParseLines[0], "\n")) != referenceHeader {
		logger.Fatalf("Header of the queue state file %s does not match expected header. File is not generated correctly.", runtimeConfig.queueStateFilePath)
	}

	logger.Debugf("Header check passed, discarding header.")
	queueStateToParseLines = queueStateToParseLines[1:]

	// Let's start parsing the file.

	/*
	 * Before starting to parse, I'll get a list of partitions that we have in our map.
	 * The idea is to only update the information of the partitions we already have,
	 * and not add any partitions to the list which user can't submit.
	 */
	availablePartitions := make([]string, 0, len(*partitionsToUpdate))

	for key := range *partitionsToUpdate {
		availablePartitions = append(availablePartitions, key)
	}

	for lineNumber, line := range queueStateToParseLines {
		// Get the line, divide to fields.
		lineFields := bytes.Split(line, []byte("|"))
		if slices.Contains(availablePartitions, string(lineFields[0])) {
			logger.Debugf("Partition %s has a job at state %s with reason %s at line %d.", lineFields[0], lineFields[1], lineFields[2], lineNumber)

			// We need the struct completely at hand, because we cannot mutate what's inside the map.
			// Copying is bad, but Go requires us to do it.
			partitionToWorkOn := (*partitionsToUpdate)[string(lineFields[0])]

			switch string(lineFields[1]) {
			case "RUNNING":
				// Update the running jobs' count.
				partitionToWorkOn.runningJobsCount++
				logger.Debugf("Job at line %d is running.", lineNumber)

			case "PENDING":
				// Update the total waiting count in any case.
				partitionToWorkOn.waitingJobsCount++

				// Also update "waiting for resources" count, if that's the reason.
				if string(lineFields[2]) == "Resources" {
					partitionToWorkOn.resourceWaitingJobsCount++
					logger.Debugf("Job at line %d is pending for Resources.", lineNumber)
				}
			}
			// We always add one for total jobs count.
			partitionToWorkOn.totalJobsCount++

			// Copy the partition data back before leaving.
			(*partitionsToUpdate)[string(lineFields[0])] = partitionToWorkOn
		} else {
			// Skip the line if the line doesn't belong to one of the partitions we already have.
			logger.Debugf("Partition %s is not available in user's accessible partitions list.", lineFields[0])
		}
	}

	return fileInfo
}

/*
 * This function removes the ignored partitions from the map to prevent them from appearing.
 * Doing this is much more efficient than checking them one by one since we directly remove the
 * key without iterating the whole map.
 */
func removeIgnoredQueues(runtimeConfiguration *RuntimeConfiguration, partitionsMap *map[string]Partition, logger *zap.SugaredLogger) {
	for _, partition := range runtimeConfiguration.ignoredPartitions {
		delete(*partitionsMap, partition)
		logger.Debugf("Removed partition %s from the map.", partition)
	}
}

// This function prints
func presentInformation(runtimeConfiguration *RuntimeConfiguration, partitionsMap *map[string]Partition, queueStateFileInfo *os.FileInfo, logger *zap.SugaredLogger) {

	// Just print what we have for inspection.
	for key, value := range *partitionsMap {
		logger.Debugf("Name of the partition is %s.", key)
		logger.Debugf("Partition has %s free CPU(s).", value.idleProcessors)
		logger.Debugf("Partition has %s total CPU(s).", value.totalProcessors)
		logger.Debugf("Partition has total of %d waiting job(s).", value.waitingJobsCount)
		logger.Debugf("Partition has total of %d waiting job(s) because of resources.", value.resourceWaitingJobsCount)
		logger.Debugf("Partition has total of %s node(s).", value.totalNodes)
		logger.Debugf("Partition has a time limit of %s per job.", value.maximumTimePerJob)
		logger.Debugf("Partition requires minimum %s node(s) per job.", value.minimumNodesPerJob)
		logger.Debugf("Queue requires maximum %s node(s) per job.", value.maximumNodesPerJob)
		logger.Debugf("Nodes in this partition has %s core(s).", value.totalCoresPerNode)
		logger.Debugf("Nodes in this partition has %d%s MB of RAM per core.", value.totalMemoryPerCore, value.totalMemoryPerCoreSuffix)
	}

	// Start by creating a Table writer and settings it output target.
	// We have a table object after that point.
	partitionStateTable := table.NewWriter()
	partitionStateTable.SetOutputMirror(os.Stdout)

	// Give our table a nice title.
	partitionStateTable.SetTitle(runtimeConfiguration.tableTitle)

	// And define our column names. This library can wrap the column names the way we want.
	partitionStateTable.AppendHeader(table.Row{"Partition\nName", "CPUs\n(Free)", "CPUs\n(Total)", "Wait. Jobs\n(Resources)", "Wait. Jobs\n(Total)", "Nodes\n(Total)", "Max. Job Time\n(D-HH:MM:SS)", "Min. Nodes\nper Job", "Max. Nodes\nper Job", "Core\nper Node", "RAM (MB)\nper Core"})

	// Then add the rows with the data we have.
	for _, details := range *partitionsMap {
		partitionStateTable.AppendRow(table.Row{details.name, details.idleProcessors, details.totalProcessors, details.resourceWaitingJobsCount, details.waitingJobsCount, details.totalNodes, details.maximumTimePerJob, details.minimumNodesPerJob, details.maximumNodesPerJob, details.totalCoresPerNode, details.totalMemoryPerCore})
	}

	// Sort the table before according to partition names.
	partitionStateTable.SortBy([]table.SortBy{
		{Name: "Partition\nName", Mode: table.Asc},
	})

	// Add a footer with the last update date.
	partitionStateTable.AppendFooter(table.Row{"Last update:", (*queueStateFileInfo).ModTime().Format(time.RFC822)})

	// Let us look fancier:
	partitionStateTable.SetStyle(table.StyleColoredYellowWhiteOnBlack)

	// Let there be light!
	partitionStateTable.Render()
}

func main() {
	// Initialize Zap logger once and for all here. Because except log levels, Zap
	// doesn't support reconfiguration. For a completely reconfigurable variant,
	// there's Thales' flume (https://github.com/ThalesGroup/flume)
	zapDefaultConfigJSON := []byte(`{
	  "level": "error",
	  "encoding": "console",
	  "outputPaths": ["stdout"],
	  "errorOutputPaths": ["stderr"],
	  "encoderConfig": {
	    "messageKey": "message",
	    "levelKey": "level",
	    "timeKey": "time",
	    "levelEncoder": "capitalColor",
	    "timeEncoder":  "ISO8601"
	  }
	}`)

	// We create a runtime config and unmarshal the JSON to the data structure.
	// Zap has its own unmarshaller for this.
	var zapRuntimeConfig zap.Config
	err := json.Unmarshal(zapDefaultConfigJSON, &zapRuntimeConfig)

	if err != nil {
		panic(err)
	}

	// Initialize the logger with the configuration object we just built.
	logger := zap.Must(zapRuntimeConfig.Build())

	defer logger.Sync() // Make sure that we sync when we exit.

	// If the logger has been started, get a Sugared logger:
	sugaredLogger := logger.Sugar()
	sugaredLogger.Debugf("Logger is up.")

	// Next, create a runtime configuration and initialize it with default values.
	var runtimeConfiguration RuntimeConfiguration
	initializeDefeaultValues(&runtimeConfiguration, sugaredLogger)

	// Then, let's apply the configuration on top of it.
	readAndApplyConfiguration(&runtimeConfiguration, sugaredLogger)

	// After finishing logger initialization, we will handle our flags next.
	versionRequested := false
	flag.BoolVar(&versionRequested, "version", false, "Show version information and exit.")
	flag.BoolVar(&versionRequested, "v", false, "Show version information and exit.")

	// Let's parse the flags and see what we have.
	flag.Parse()

	// If user wants to see the version, just show it and exit cleanly.
	if versionRequested {
		// Clean the path information which might be appended during the execution
		// to make the output consistent and parseable.
		sugaredLogger.Debugf("Version requested, showing and exiting.")
		applicationName := strings.Split(os.Args[0], "/")
		fmt.Printf("%s version %s\n", applicationName[len(applicationName)-1], runtimeConfiguration.softwareVersion)
		os.Exit(0)
	}

	// Start by getting the partitions & the raw information via sinfo.
	partitionInformation := getPartitionsInformation(sugaredLogger)

	// Let's look what we have returned.
	sugaredLogger.Debugf("Partition information returned as follows:\n%s", partitionInformation)

	// Let's get the data that we need.
	partitionsMap := parsePartitionsInformation(partitionInformation, sugaredLogger)
	queueStateFileInfo := parseQueueStateFile(&runtimeConfiguration, &partitionsMap, sugaredLogger)
	removeIgnoredQueues(&runtimeConfiguration, &partitionsMap, sugaredLogger)
	presentInformation(&runtimeConfiguration, &partitionsMap, &queueStateFileInfo, sugaredLogger)
}
