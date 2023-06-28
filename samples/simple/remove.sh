#!/bin/bash -u

# Copyright 2018 ConsenSys AG.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
# the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
# an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
# specific language governing permissions and limitations under the License.

NO_LOCK_REQUIRED=false

. ./.env
. ./.common.sh

removeDockerImage(){
  if [[ ! -z `docker ps -a | grep $1` ]]; then
    docker image rm $1
  fi
}

echo "${bold}*************************************"
echo "ETH 1 Testbench "
echo "*************************************${normal}"
echo "Stop and remove network..."

docker-compose down -v
docker-compose rm -sfv

# Remove validator images
docker rmi $(docker images | grep validator | tr -s ' ' | cut -d ' ' -f 3) -f

if [ -f "docker-compose-deps.yml" ]; then
    echo "Stopping dependencies..."
    docker-compose -f docker-compose-deps.yml down -v
    docker-compose rm -sfv
fi

rm ${LOCK_FILE}
echo "Lock file ${LOCK_FILE} removed"
