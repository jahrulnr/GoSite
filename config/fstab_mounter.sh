#!/bin/bash

# This script will mount all filesystems defined in /etc/fstab after ensuring the mount points exist.

echo "Checking and mounting all filesystems defined in /etc/fstab..."

# Read each line in /etc/fstab
while read -r line; do
    # Skip comments and empty lines
    if [[ "$line" == \#* ]] || [[ -z "$line" ]]; then
        continue
    fi
    
    # Extract the second column (mount point) using awk
    mountpoint=$(echo "$line" | awk '{print $2}')
    
    # Check if the mount point directory exists
    if [ ! -d "$mountpoint" ]; then
        echo "Mount point $mountpoint does not exist. Creating..."
        mkdir -p "$mountpoint"
        if [ $? -ne 0 ]; then
            echo "Failed to create mount point $mountpoint. Skipping..."
            continue
        fi
    fi

    # Extract the device and filesystem type
    device=$(echo "$line" | awk '{print $1}')
    fstype=$(echo "$line" | awk '{print $3}')

    # Now attempt to mount the filesystem
    echo "Mounting $device on $mountpoint..."
    mount $mountpoint
    if [ $? -eq 0 ]; then
        echo "Mounted $device on $mountpoint successfully."
    else
        echo "Failed to mount $device on $mountpoint."
    fi
done < <(cat "/etc/fstab"; echo)

echo "All filesystems processing complete."
