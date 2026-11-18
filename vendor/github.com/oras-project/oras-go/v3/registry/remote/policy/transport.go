/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package policy

// TransportName represents a supported transport type
type TransportName string

const (
	// TransportNameDocker represents the docker transport
	TransportNameDocker TransportName = "docker"
	// TransportNameAtomic represents the atomic transport
	TransportNameAtomic TransportName = "atomic"
	// TransportNameContainersStorage represents the containers-storage transport
	TransportNameContainersStorage TransportName = "containers-storage"
	// TransportNameDir represents the dir transport
	TransportNameDir TransportName = "dir"
	// TransportNameDockerArchive represents the docker-archive transport
	TransportNameDockerArchive TransportName = "docker-archive"
	// TransportNameDockerDaemon represents the docker-daemon transport
	TransportNameDockerDaemon TransportName = "docker-daemon"
	// TransportNameOCI represents the oci transport
	TransportNameOCI TransportName = "oci"
	// TransportNameOCIArchive represents the oci-archive transport
	TransportNameOCIArchive TransportName = "oci-archive"
	// TransportNameSIF represents the sif transport
	TransportNameSIF TransportName = "sif"
	// TransportNameTarball represents the tarball transport
	TransportNameTarball TransportName = "tarball"
)
