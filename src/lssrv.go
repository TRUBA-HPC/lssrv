/*
 * List free servers
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
	"strconv"
	"encoding/json"
	"go.uber.org/zap"
	"os"
	"os/exec"
)

// This is our basic data structure which holds everything about a SLURM partition
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
	totalMemoryPerNode string // This is RAM per node, in megabytes.
	totalMemoryPerCore uint   // This contains RAM per core, in megabytes.
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

// This function runs sinfo command with an hard coded argument list, because we need
// an output with the format we specified.
func getPartitions(logger *zap.SugaredLogger) []byte {
	logger.Debugf("getPartitions reporting in.")
	//TODO: Run sinfo here, get the output, store in an array line by line, and return.

	// Check whether we have the command in the OS, so we can continue safely.
	path, err := exec.LookPath("sinfo")

	if err != nil {
		// And exit gracefully if we don't have that.
		logger.Fatalf("Cannot find sinfo executable (error is %s), exiting.", err)
		os.Exit(1)
	}

	logger.Debugf("sinfo is found at %s.", path)

	// Build a command object so we can run it.
	commandToRun := exec.Command(path, "--format=%R|%a|%D|%B|%c|%C|%s|%z|%l|%m")

	// Let it run!
	logger.Debugf("Will be running the command %s with args %s.", commandToRun.Path, commandToRun.Args)
	partitionInformation, err := commandToRun.Output()

	if err != nil {
		logger.Fatalf("sinfo command returned an error (error is %s)", err)
	}

	// We have an extra line at the end of the output, let's trim this.
	partitionInformation = bytes.TrimRight(partitionInformation, "\n")

	logger.Debugf("Got the partition information, returning.")
	return partitionInformation
}

func parsePartitionInformation(partitionInformation []byte, logger *zap.SugaredLogger) map[string]Partition {
	// We'll start by dividing the partition information to lines.
	partitionInformationLines := bytes.Split(partitionInformation, []byte("\n"))

	// Let's see what we got.
	logger.Debugf("Partition information has returned %d line(s).", len(partitionInformationLines))

	/*
	 * The idea here is to compare the header with a known good value to check that
	 * everything is formatted the way we want. Then we can continue parsing the rest.
	 */
	logger.Debugf("Checking command output, comparing headers.")

	referenceHeader := "PARTITION|AVAIL|NODES|MAX_CPUS_PER_NODE|CPUS|CPUS(A/I/O/T)|JOB_SIZE|S:C:T|TIMELIMIT|MEMORY"

	if string(partitionInformationLines[0]) != referenceHeader {
		logger.Errorf("Command output header doesn't match required template. This version of sinfo is not compatible, exiting.")
		os.Exit(1)
	}

	logger.Debugf("Header check is passed.")

	// Discard the header since we don't need that anymore.
	partitionInformationLines = partitionInformationLines[1:]
	logger.Debugf("Partition information now has %d line(s).", len(partitionInformationLines))

	// Create a map to hold the partitions, indexed by their name (hold that thought).
	partitionMap := make(map[string]Partition)

	// Start traversing the partitions we have.
	for index, value := range partitionInformationLines {
		logger.Debugf("Partition #%d is %s.", index, value)

		// And split them according to the divider we selected before.
		partitionFields := bytes.Split(value, []byte("|"))
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

		// Now we will parse the processor counts per queue. It's stored as "Allocated/Idle/Other/Total".
		processorCounts := bytes.Split(partitionFields[5], []byte("/"))
		partitionToParse.allocatedProcessors = string(processorCounts[0])
		partitionToParse.idleProcessors = string(processorCounts[1])
		partitionToParse.otherProcessors = string(processorCounts[2])
		partitionToParse.totalProcessors = string(processorCounts[3])
		logger.Debugf("Processor counts for partition is %s/%s/%s/%s (Available/Idle/Other/Total).", partitionToParse.allocatedProcessors, partitionToParse.idleProcessors, partitionToParse.otherProcessors, partitionToParse.totalProcessors)

		// Next we'll handle the node count limitations per job. Format is "minimum-maximum"
		nodeCounts := bytes.Split(partitionFields[6], []byte("-"))
		partitionToParse.minimumNodesPerJob = string(nodeCounts[0])
		partitionToParse.maximumNodesPerJob = string(nodeCounts[1])
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
		totalMemoryPerNode, err := strconv.ParseUint(string(memoryInformation[0]), 10, 64)
		
		if err != nil {
			logger.Fatalf("Cannot convert total memory amount to uint (error is %s).", err)
		}
		
		// We also need to handle the "+" suffix if the partition we're working on is heterogenous.
		totalCoreCountPerNode, err := strconv.ParseUint(string(bytes.Split(partitionFields[4], []byte("+"))[0]), 10, 64)
		
		if err != nil {
			logger.Fatalf("Cannot convert total core count to uint (error is %s).", err)
		}
		
		partitionToParse.totalMemoryPerCore = uint(totalMemoryPerNode) / uint(totalCoreCountPerNode)
		logger.Debugf("This partition has %dMB of RAM per core.", partitionToParse.totalMemoryPerCore)

		// Add the completed partition to the map.
		partitionMap[partitionToParse.name] = partitionToParse
	}
	
	return partitionMap
}

// Following function parses the queue state function and adds the information to the relevant partition.
// It basically parses the file line by line and counts the job states.
func parseQueueState (partitionsToUpdate *map[string]Partition, queueStateFilePath string, logger *zap.SugaredLogger) {
	// As a good, defensive programmer, we need to make sure that the 
}

func main() {
	// Initialize Zap logger once and for all here. Because except log levels, Zap
	// doesn't support reconfiguration. For a completely reconfigurable variant,
	// there's Thales' flume (https://github.com/ThalesGroup/flume)
	zapDefaultConfigJSON := []byte(`{
	  "level": "debug",
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

	sugaredLogger.Infof("Hello, world!")

	// Start by getting the partitions & the raw information via sinfo.
	partitionInformation := getPartitions(sugaredLogger)

	// Let's look what we have returned.
	sugaredLogger.Debugf("Partition information returned as follows:\n%s", partitionInformation)

	parsePartitionInformation(partitionInformation, sugaredLogger)
}
