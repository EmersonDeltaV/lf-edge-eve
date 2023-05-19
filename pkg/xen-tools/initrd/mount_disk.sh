#!/bin/sh

# we expect the same order in block device enumeration (we put them in order in VM's configuration)
# and in /mnt/mountPoints file (where mount points defined)

mountPointLineNo=1
find /sys/block/ -maxdepth 1 -regex '.*[sv]d.*' -exec basename '{}' ';'| sort | while read -r disk ; do
  echo "Processing $disk"
  targetDir=$(sed "${mountPointLineNo}q;d" /mnt/mountPoints)
  if [ -z "$targetDir" ]
    then
      echo "Error while mounting: No Mount-Point found for $disk."
      exit 0
  fi

  #Checking and creating a ext4 file system inside the partition
  fileSystem="ext4"
  # We only care if filesystem exists, not what type it is. So just check for TYPE existence.
  existingFileSystem="$(blkid "/dev/$disk" | grep TYPE= )"
  if [ "$existingFileSystem" = "" ]; then
    echo "Creating $fileSystem file system on /dev/$disk"
    mke2fs -t $fileSystem "/dev/$disk" && \
    echo "Successfully created $fileSystem file system on /dev/$disk" || \
    echo "Failed to create $fileSystem file system on /dev/$disk"
    echo
  fi

  #Mounting the partition onto a target directory
  diskAccess=$(cat "/sys/block/$disk/ro")
  if [ "$diskAccess" -eq 0 ]; then
    accessRight=rw
  else
    accessRight=ro
  fi
  stage2TargetPath="/mnt/rootfs$targetDir"
  echo "Mounting /dev/$disk on $stage2TargetPath with access: $accessRight"
  mkdir -p "$stage2TargetPath"
  mount -O remount,$accessRight "/dev/$disk" "$stage2TargetPath" && \
  echo "Successfully mounted file system:/dev/$disk on $targetDir" || \
  echo "Failed to mount file system:/dev/$disk on $targetDir"

  mountPointLineNo=$((mountPointLineNo + 1))
done
