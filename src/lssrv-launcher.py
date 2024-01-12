#!/bin/bash

# This small batch file autoinstalls the requirements and fires up the tool for us.

# Load the required Python 3.8 version.
module load centos7.3/comp/python/3.8.12-openmpi-4.1.1-oneapi-2021.2

# Is Rich installed, let's look silently.
if ! pip3 -qqq show rich; then
    # Looks like no. Let's silently install.
    pip3 -qqq install rich --user --no-python-version-warning
    
    # Then, run the tool.
    python3 /usr/local/bin/lssrv.py
else
    # Looks like we can directly start.
    python3 /usr/local/bin/lssrv.py
fi

# Leave the way you started.
module unload centos7.3/comp/python/3.8.12-openmpi-4.1.1-oneapi-2021.2 
