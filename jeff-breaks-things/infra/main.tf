module "big_bang" {
  source = "git::https://repo1.dso.mil/platform-one/big-bang/terraform-modules/big-bang-terraform-launcher.git"

  kube_conf_file = "/etc/rancher/k3s/k3s.yaml"
  big_bang_manifest_file = "start.yaml"
  registry_credentials = [{
    registry = "registry1.dso.mil"
    username = ""
    password = ""
  }]
}
