#!/bin/sh

config='config.yaml'
# The Oracle container takes sometime to start....
timeout=60

if [ -f $config ] 
then

  sleep $timeout
  echo "Connecting to DB...."
  $(./grafeas-server --config $config)

else
  echo "No config file is specified, exiting" 1>&2
  exit 1
fi

echo "Done"