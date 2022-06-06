#!/usr/bin/env python3
# -*- coding: utf-8 -*-

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
@version: 0.0.5a20220509
'''

import os
import logging
import subprocess

from datetime import datetime

import argparse
import configparser

# Rich related imports come here.
from rich.console import Console
from rich.table import Table
from rich.markdown import Markdown

# Code constants. Moved here for easier code maintenance and debugging.

VERSION = '0.0.5a20220606'
LOG_LEVEL = logging.ERROR
LOG_FILE_PATH = None

class Partition:
	'''
	@summary: This object contains every partition in the cluster and every detail accessible by the user.
	'''
	
	def __init__ (self):
		''' Default constructor for the class.'''
		pass
	
	def __init__ (self, scontrol_output_string):
		'''A constructor which gets the scontrol output and initializes the class.'''
		
		# Get a class wide logger.
		logger = logging.getLogger('Partition_class')
		
		# and create an intermediate string for processing partition properties.
		partition_property_bundle = scontrol_output_string.split(b' ')
		
		# Will fill properties to a dictionary and will transfer to variables then.
		for property in partition_property_bundle:
			
			logger.debug('Processing feature: %s.', str(property))
			
			# Extract the property name and value.
			property_name_and_value = property.split(b'=')
			
			# Directly create a new variable with the string coming from the property list.
			setattr(self, property_name_and_value[0].decode('utf-8'), property_name_and_value[1].decode('utf-8'))
		
		logger.debug('Extracted partition properties: %s.', str(dir(self)))
		
		# Also, create partition load variables
		self.pending_jobs_per_category = dict()
		self.pending_jobs_per_category['Resources'] = 0 # We will query this anyway, and sometimes there are no pending jobs, so its absence creates problems.
		self.pending_jobs_total = 0 # Storing total here allows us to gain some important performance while showing the table.
		self.busy_cpu_count = 0
		
		# Check whether partition has more than one type of servers.
		# TODO: Fix this code. If there are two chunks of same server type, it makes mistakes.
		server_types = self.Nodes.split(',')
			
		if len(server_types) > 1:
			logger.debug('Partition has more than one server type.')
			self.homogenous = False
		else:
			logger.debug('Partition has one server type.')
			self.homogenous = True
		

def get_all_partitions():
	''' This functions ask Slurm for all partitions, then creates & returns several partition objects in a dictionary.'''
	
	logger = logging.getLogger('get_all_partitions')
	
	# Get the partitions directly from Slurm.
	# Line is ending with \n. Always strip before splitting.
	partitions = subprocess.check_output(['scontrol', 'show', 'partition', '-o']).strip().split(b'\n')
	
	logger.info('Have read %d partition(s).', len(partitions))
	
	# Create a list for storing partition objects.
	partitions_dictionary = dict()
	
	for partition in partitions:
		temporary_partition = Partition(partition)
		partitions_dictionary[temporary_partition.PartitionName] = temporary_partition
	
	logger.debug('Returning %d partition(s).', len(partitions))
	return partitions_dictionary

def get_job_state_for_partitions(partitions, partitions_to_ignore, queue_state_file_path):
	''' This function gets all the jobs and updates the job state information per partition.'''
	
	logger = logging.getLogger('get_job_state_for_partitions')
	
	# This is a very expensive command in terms of time. Calling it once and processing more is much more preferable to calling it 10+ times.
	if os.path.exists(queue_state_file_path):
		queue_state = open(queue_state_file_path, 'r')
		jobs_in_the_cluster = queue_state.readlines()
		queue_state.close()
	
	#TODO: Add exception handling here. Sometimes previous command returns nothing. 
	for job in jobs_in_the_cluster:
		logger.debug('Job line to process is %s.', job)
		splitted_line = job.split()
		job_partition = splitted_line[0].strip()
		job_core_count = splitted_line[1].strip()
		job_state = splitted_line[2].strip()
		job_reason = splitted_line[3].strip()

		# If the partition is in the ignore list, just move along. There's nothing to see/do here.
		if job_partition in partitions_to_ignore:
			continue
		
		# Sometimes a user has no access to a partition, but we have the information anyways, and it creates problems.
		if job_partition not in partitions:
			continue

		# Parse the job state and fill relevant information accordingly.
		# The following logging lines use %s for 'job_core_count' because they arrive as strings already.
		if job_state == 'RUNNING':
			logger.debug('Job is running with %s core(s) on partition %s.', job_core_count, job_partition)
			partitions[job_partition].busy_cpu_count = partitions[job_partition].busy_cpu_count + int(job_core_count)
		
		elif job_state == 'PENDING':
			logger.debug('Job is waiting with a request for %s core(s) on partition %s, because of %s.', job_core_count, job_partition, job_reason)
			
			# Let's check whether whether we already have a counter for the reason at hand. If not, create one.
			if job_reason in partitions[job_partition].pending_jobs_per_category:
				logger.debug('Incrementing key %s for partition %s by 1.', job_reason, job_partition)
				partitions[job_partition].pending_jobs_per_category[job_reason] = partitions[job_partition].pending_jobs_per_category[job_reason] + 1 
			else:
				logger.debug('Creating key %s for partition %s and setting it to 1.', job_reason, job_partition)
				partitions[job_partition].pending_jobs_per_category[job_reason] = 1
				
			# This needs to be incremented anyway:
			partitions[job_partition].pending_jobs_total = partitions[job_partition].pending_jobs_total + 1
			
if __name__ == '__main__':
	 # Let's parse some arguments.
	argument_parser = argparse.ArgumentParser()
	argument_parser.description = 'A tool to see partitions\' state in the cluster.'

	# Version always comes last.
	argument_parser.add_argument ('-V', '--version', help='Print ' + argument_parser.prog + ' version and exit.', action='version', version=argument_parser.prog + ' version ' + VERSION)    

	arguments = argument_parser.parse_args()
	
	# Set up simple logging:
	logging.basicConfig(filename=LOG_FILE_PATH, level=LOG_LEVEL)
	
	logger = logging.getLogger('main')
	
	# Create a configuration parser and parse the configuration file.
	configuration = configparser.ConfigParser()
	
	# Check whether we can read the configuration file, and proceed accordingly:
	# The nice thing about exists() is, it fails if I can't read the file too. 
	if os.path.exists('/etc/lssrv.conf'):
		configuration.read('/etc/lssrv.conf')
	
		try:
			partitions_to_ignore = configuration['Partitions']['partitions_to_hide'].strip().split()
			
		except KeyError as exception:
			logger.debug('Cannot find configuration value for partitions to ignore, using defaults.')
			partitions_to_ignore = list() # The default value is an empty list for that.	
		
		logger.debug('Got %d partitions to ignore.', len(partitions_to_ignore))
		logger.debug('Partition(s) to ignore: %s.', str(partitions_to_ignore))
		
		try:		
			queue_state_file_path = configuration['General']['squeue_state_file_path'].strip()
		except KeyError as exception:
			logger.debug('Cannot find configuration value for squeue state file, using default.')
			queue_state_file_path = '/var/cache/lssrv/squeue.state' # Use standard file path conventions, keep the system tidy.
			
		logger.debug('squeue state file is: %s.', queue_state_file_path)	
	else:
		# Loading all the defaults automatically.
		partitions_to_ignore = list() # The default value is an empty list for that.
		queue_state_file_path = '/var/cache/lssrv/squeue.state' # Use standard file path conventions, keep the system tidy. 
		
	
	# Create a rich console.
	console = Console()
	
	with console.status("Please wait while gathering resource information...", spinner = 'line'):
		all_partitions = get_all_partitions()
		get_job_state_for_partitions(all_partitions, partitions_to_ignore, queue_state_file_path)
		stateFileLastUpdateTime = os.path.getmtime(queue_state_file_path)
	
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
	cores_per_node = 0
	
	# This is where we add the information we have
	for partition_name, partition_details in all_partitions.items():
		
		# If this partition is one of the ones we've been directed to ignore...
		if partition_name in partitions_to_ignore:
			continue # Just go on, nothing to see here. 
		
		if partition_details.homogenous == True:
			cores_per_node = str(int(int(partition_details.TotalCPUs) / int(partition_details.TotalNodes))) # Yes, we need all these conversions. All variables are stored as strings, and default Python division is double. So, yes.
		else:
			cores_per_node = "-"
		
		table.add_row(partition_name, str(int(partition_details.TotalCPUs) - partition_details.busy_cpu_count), partition_details.TotalCPUs, str(partition_details.pending_jobs_per_category['Resources']), str(partition_details.pending_jobs_total), partition_details.TotalNodes, partition_details.MaxTime, partition_details.MinNodes, partition_details.MaxNodes, cores_per_node, partition_details.DefMemPerCPU + ' MB')
	
	
	console.print(table)
	last_update_time_markdown = Markdown('**Last update:** ' + datetime.fromtimestamp(stateFileLastUpdateTime).strftime('%Y-%m-%d %H:%M:%S'))
	console.print(last_update_time_markdown)
