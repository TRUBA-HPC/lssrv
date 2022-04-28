#!/usr/bin/env python3

'''
List free servers
Copyright (C) 2022  Hakan Bayındır

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.

@author: Hakan Bayindir
@contact: hakan.bayindir@tubitak.gov.tr
@license: GNU/GPLv3
@version: 0.0.4
'''

import os
import logging
import subprocess

from datetime import datetime

import configparser

# Rich related imports come here.
from rich.console import Console
from rich.table import Table
from rich.markdown import Markdown

class Partition:
	'''
	@summary: This object contains every partition in the cluster and every detail accessible by the user.
	'''
	
	def __init__ (self):
		''' Default constructor for the class.'''
		pass
	
	def __init__ (self, scontrolOutputString):
		'''A constructor which gets the scontrol output and initializes the class.'''
		
		# Get a class wide logger.
		logger = logging.getLogger('Partition_class')
		
		# and create an intermediate string for processing partition properties.
		partitionPropertyBundle = scontrolOutputString.split(b' ')
		
		# Will fill properties to a dictionary and will transfer to variables then.
		for property in partitionPropertyBundle:
			
			logger.debug('Processing feature: ' + str(property))
			
			# Extract the property name and value.
			propertyNameAndValue = property.split(b'=')
			
			# Directly create a new variable with the string coming from the property list.
			setattr(self, propertyNameAndValue[0].decode('utf-8'), propertyNameAndValue[1].decode('utf-8'))
		
		logger.debug('Extracted partition properties: ' + str(dir(self)))
		
		# Also, create partition load variables
		self.pendingJobsPerCategory = dict()
		self.pendingJobsPerCategory['Resources'] = 0 # We will query this anyway, and sometimes there are no pending jobs, so its absence creates problems.
		self.pendingJobsTotal = 0 # Storing total here allows us to gain some important performance while showing the table.
		self.busyCPUCount = 0
		
		# Check whether partition has more than one type of servers.
		# TODO: Fix this code. If there are two chunks of same server type, it makes mistakes.
		serverTypes = self.Nodes.split(',')
			
		if len(serverTypes) > 1:
			logger.debug('Partition has more than one server type.')
			self.homogenous = False
		else:
			logger.debug('Partition has one server type.')
			self.homogenous = True
		

def getAllPartitions():
	''' This functions ask Slurm for all partitions, then creates & returns several partition objects in a dictionary.'''
	
	logger = logging.getLogger('getAllPartitions')
	
	# Get the partitions directly from Slurm.
	# Line is ending with \n. Always strip before splitting.
	partitions = subprocess.check_output(['scontrol', 'show', 'partition', '-o']).strip().split(b'\n')
	
	logger.info('Have read ' + str(len(partitions)) + ' partition(s).')
	
	# Create a list for storing partition objects.
	partitionsDict = dict()
	
	for partition in partitions:
		tempPartition = Partition(partition)
		partitionsDict[tempPartition.PartitionName] = tempPartition
	
	logger.debug('Returning ' + str(len(partitions)) + ' partition(s).')
	return partitionsDict

def getJobStateForPartitions(partitions, partitionsToIgnore, queueStateFilePath):
	''' This function gets all the jobs and updates the job state information per partition.'''
	
	logger = logging.getLogger('getJobStateForPartitions')
	
	# This is a very expensive command in terms of time. Calling it once and processing more is much more preferable to calling it 10+ times.
	if os.path.exists(queueStateFilePath):
		queueState = open(queueStateFilePath, 'r')
		jobsInTheCluster = queueState.readlines()
		queueState.close()
	
	#TODO: Add exception handling here. Sometimes previous command returns nothing. 
	for job in jobsInTheCluster:
		splittedLine = job.split()
		jobPartition = splittedLine[0].strip()
		jobCoreCount = splittedLine[1].strip()
		jobState = splittedLine[2].strip()
		jobReason = splittedLine[3].strip()

		# If the partition is in the ignore list, just move along. There's nothing to see/do here.
		if jobPartition in partitionsToIgnore:
			continue
		
		# Sometimes a user has no access to a partition, but we have the information anyways, and it creates problems.
		if jobPartition not in partitions:
			continue

		# Parse the job state and fill relevant information accordingly.
		if jobState == 'RUNNING':
			logger.debug('Job is running with ' + jobCoreCount + 'core(s) on partition ' + jobPartition + '.')
			partitions[jobPartition].busyCPUCount = partitions[jobPartition].busyCPUCount + int(jobCoreCount)
		
		elif jobState == 'PENDING':
			logger.debug('Job is running with ' + jobCoreCount + 'core(s) on partition ' + jobPartition + ', because of ' + jobReason + '.')
			
			# Let's check whether whether we already have a counter for the reason at hand. If not, create one.
			if jobReason in partitions[jobPartition].pendingJobsPerCategory:
				logger.debug('Incrementing key ' + jobReason + ' for partition ' + jobPartition + ' by 1.')
				partitions[jobPartition].pendingJobsPerCategory[jobReason] = partitions[jobPartition].pendingJobsPerCategory[jobReason] + 1 
			else:
				logger.debug('Creating key ' + jobReason + ' for partition ' + jobPartition + ' and setting it to 1.')
				partitions[jobPartition].pendingJobsPerCategory[jobReason] = 1
				
			# This needs to be incremented anyway:
			partitions[jobPartition].pendingJobsTotal = partitions[jobPartition].pendingJobsTotal + 1
			
if __name__ == '__main__':
	# Set up simple logging:
	logging.basicConfig(filename = None, level=logging.ERROR)
	
	logger = logging.getLogger('main')
	
	# Create a configuration parser and parse the configuration file.
	configuration = configparser.ConfigParser()
	
	# Check whether we can read the configuration file, and proceed accordingly:
	# The nice thing about exists() is, it fails if I can't read the file too. 
	if os.path.exists('/etc/lssrv.conf'):
		configuration.read('/etc/lssrv.conf')
	
		try:
			partitionsToIgnore = configuration['Partitions']['partitions_to_hide'].strip().split()
			
		except KeyError as exception:
			logger.debug('Cannot find configuration value for partitions to ignore, using defaults.')
			partitionsToIgnore = list() # The default value is an empty list for that.	
		
		logger.debug('Got ' + str(len(partitionsToIgnore)) + ' partitions to ignore.')
		logger.debug('Partition(s) to ignore: ' + str(partitionsToIgnore))
		
		try:		
			queueStateFilePath = configuration['General']['squeue_state_file_path'].strip()
		except KeyError as exception:
			logger.debug('Cannot find configuration value for squeue state file, using default.')
			queueStateFilePath = '/var/cache/lssrv/squeue.state' # Use standard file path conventions, keep the system tidy.
			
		logger.debug('squeue state file is: ' + str(queueStateFilePath))	
	else:
		# Loading all the defaults automatically.
		partitionsToIgnore = list() # The default value is an empty list for that.
		queueStateFilePath = '/var/cache/lssrv/squeue.state' # Use standard file path conventions, keep the system tidy. 
		
	
	# Create a rich console.
	console = Console()
	
	with console.status("Please wait while gathering resource information...", spinner = 'line'):
		allPartitions = getAllPartitions()
		getJobStateForPartitions(allPartitions, partitionsToIgnore, queueStateFilePath)
		stateFileLastUpdateTime = os.path.getmtime(queueStateFilePath)
	
	# Let's start building our table.
	table = Table(title = 'TRUBA Partitions State')
	table.add_column('Partition\nName')
	table.add_column('CPUs\n(Free)')
	table.add_column('CPUs\n(Total)')
	table.add_column('Wait. Jobs\n(Resource)')
	table.add_column('Wait. Jobs\n(Total)')
	table.add_column('Nodes\n(Total)')
	table.add_column('Max Job Time\nDD-HH:MM:SS', justify = 'right')
	table.add_column('Min. Nodes\nPer Job', justify = 'right')
	table.add_column('Max. Nodes\nPer Job', justify = 'right')
	table.add_column('Core\nper Node')
	table.add_column('RAM\nper Core')
	
	# These variables are created here to prevent re-creation in loop continuously.
	coresPerNode = 0
	
	# This is where we add the information we have
	for partitionName, partitionDetails in allPartitions.items():
		
		# If this partition is one of the ones we've been directed to ignore...
		if partitionName in partitionsToIgnore:
			continue # Just go on, nothing to see here. 
		
		if partitionDetails.homogenous == True:
			coresPerNode = str(int(int(partitionDetails.TotalCPUs) / int(partitionDetails.TotalNodes))) # Yes, we need all these conversions. All variables are stored as strings, and default Python division is double. So, yes.
		else:
			coresPerNode = "-"
		
		table.add_row(partitionName, str(int(partitionDetails.TotalCPUs) - partitionDetails.busyCPUCount), partitionDetails.TotalCPUs, str(partitionDetails.pendingJobsPerCategory['Resources']), str(partitionDetails.pendingJobsTotal), partitionDetails.TotalNodes, partitionDetails.MaxTime, partitionDetails.MinNodes, partitionDetails.MaxNodes, coresPerNode, partitionDetails.DefMemPerCPU + ' MB')
	
	
	console.print(table)
	lastUpdateTimeMarkdown = Markdown('**Last update:** ' + datetime.fromtimestamp(stateFileLastUpdateTime).strftime('%Y-%m-%d %H:%M:%S'))
	console.print(lastUpdateTimeMarkdown)
