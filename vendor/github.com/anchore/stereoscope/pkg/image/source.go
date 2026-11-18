package image

type Source = string

const (
	UnknownSource           Source = ""
	ContainerdDaemonSource  Source = "containerd"
	ContainersStorageSource Source = "containers-storage"
	DockerTarballSource     Source = "docker-archive"
	DockerDaemonSource      Source = "docker"
	OciDirectorySource      Source = "oci-dir"
	OciTarballSource        Source = "oci-archive"
	OciRegistrySource       Source = "oci-registry"
	PodmanDaemonSource      Source = "podman"
	SingularitySource       Source = "singularity"
)
