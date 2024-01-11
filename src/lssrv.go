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
	"encoding/json"
	"go.uber.org/zap"
)

// This is our basic data structure which holds everything about a SLURM partition
type Partition struct {
	// Basic information for this partition.
	name  string //Name of the partition
	state string // Normally a partition can be up or down. So it may change to bool later.

	// Following are the stats about our processors.
	totalProcessors     uint // Total number of processors in this partition.
	allocatedProcessors uint // Number of allocated processors in this partition.
	idleProcessors      uint // Number of idle processors in this partition.
	otherProcessors     uint // Number of processors in "other" state in this partition.

	// Following are related about nodes in the partition.
	totalNodes uint // Total number of nodes (servers) in this partition.

	// Processor geometry per node in this partition.
	socketsPerNode uint // How many CPU sockets a node have.
	coresPerSocket uint // How many cores we have per CPU socket.
	threadsPerCore uint // How many threads a core can execute (It's two for HyperThreading/SMT systems).
	totalCores     uint // How many cores we have in total, per node.

	// Nodes' memory is also a concern for us.
	memoryPerNode uint // This is RAM per node, in megabytes.
	memoryPerCore uint // This contains RAM per core, in megabytes.

	// Job parameters for this partition.
	minimumNodesPerJob uint // Minimum number of nodes we can allocate per job.
	maximumNodesPerJob int  // Maximum number of nodes we can allocate per job. -1 means infinite.

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
}
