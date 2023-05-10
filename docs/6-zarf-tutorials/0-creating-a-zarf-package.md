# Creating a Zarf Package

## Introduction

In this tutorial, we will demonstrate the process to create a Zarf package for an application from defining a `zarf.yaml`, finding resources with `zarf prepare` commands and finally building the package with `zarf package create`.

When creating a Zarf package, you must have a network connection so that Zarf can fetch all of the dependencies and resources necessary to build the package. If your package is using images from a private registry or is referencing repositories in a private repository, you will need to have your credentials configured on your machine for Zarf to be able to fetch the resources.

## System Requirements

- You'll need an internet connection so Zarf can pull in anything required to build the package in this tutorial.

## Prerequisites

Before beginning this tutorial you will need the following:

-  Zarf binary installed on your $PATH: ([Installing Zarf](../1-getting-started/index.md#installing-zarf))
-  A text editor or development environment such as [VS Code](../3-create-a-zarf-package/8-vscode.md)

## Putting Together a Zarf Package

In order to create a Zarf package you first need to have an idea of what application(s) you want to package.  In this example we will be using the [Wordpress chart from Bitnami](https://artifacthub.io/packages/helm/bitnami/wordpress) but the steps and tools used below are very similar for other applications.

### Creating the Package Definition

A `zarf.yaml` file follows the [Zarf Package Schema](../3-create-a-zarf-package/4-zarf-schema.md) and allows us to specify package metadata and a set of components for us to deploy. We start a package definition with the `kind` of package we are making and `metadata` that describes the package.  You can start our Wordpress package by creating a new `zarf.yaml` with the following content:

```yaml
kind: ZarfPackageConfig # ZarfPackageConfig is the package kind for most normal zarf packages
metadata:
  name: wordpress       # specifies the name of our package and should be unique and unchanging through updates
  version: 16.0.4       # (optional) a version we can track as we release updates or publish to a registry
  description: |        # (optional) a human-readable description of the package that you are creating
    "A Zarf Package that deploys the Wordpress blogging and content management platform"
```

:::tip

If you are using an Integrated Development Environment (such as [VS Code](../3-create-a-zarf-package/8-vscode.md)) to create and edit the `zarf.yaml` file, you can install or reference the [`zarf.schema.json`](https://github.com/defenseunicorns/zarf/blob/main/zarf.schema.json) file to get error checking and autocomplete.

:::

### Adding the Wordpress Component

Components are the unit of Zarf Packages that define an application stack.  These are defined under the `components` key and allow many different resource types to be brought into a package.  You can learn more about components on the [Understanding Zarf Components](../3-create-a-zarf-package/2-zarf-components.md) page. To add our Wordpress component, add the following to the bottom of our `zarf.yaml`:

```yaml
components:
  - name: wordpress  # specifies the name of our component and should be unique and unchanging through updates
    description: |   # (optional) a human-readable description of the component you are defining
      "Deploys the Bitnami-packaged Wordpress chart into the cluster"
    required: true   # (optional) sets the component as 'required' so that it is always deployed
    charts:
      - name: wordpress
        url: oci://registry-1.docker.io/bitnamicharts/wordpress
        version: 16.0.4
        namespace: wordpress
        valuesFiles:
          - wordpress-values.yaml
```

In addition to this component definition, we also need to create the `valuesFiles` we have specified.  In this case we need to create a file named `wordpress-values.yaml` in the same directory as our `zarf.yaml` with the following contents:

```yaml
# We are hard-coding these for now but will make them dynamic in Setting up Variables.
wordpressUsername: zarf
wordpressPassword: ""
wordpressEmail: hello@defenseunicorns.com
wordpressFirstName: Zarf
wordpressLastName: The Axolotl
wordpressBlogName: The Zarf Blog

# This value turns on the metrics exporter and thus will require another image.
metrics:
  enabled: true

# Sets the Wordpress service as a ClusterIP service to not conflict with potential
# pre-existing LoadBalancer services.
service:
  type: ClusterIP
```

:::note

We create any `values.yaml` file(s) at this stage because the `zarf prepare find-images` command we will use next will template out this chart to look only for the images we need.

:::

:::caution

Note that we are explicitly defining the `wordpress` namespace for this deployment, this is strongly recommended to separate out the applications you deploy and to avoid issues with the Zarf Agent not being able to mutate your resources as it intentionally ignores resources in the `default` or `kube-system` namespaces.  See [what happens to resources that exist before Zarf init](../8-faq.md#what-happens-to-resources-that-exist-in-the-cluster-before-zarf-init) for more information.

:::

### Finding the Images

Once you have the above defined we can now work on setting the images that we will need to bring with us into the air gap.  For this, Zarf has a helper command you can run with `zarf prepare find-images`.  Running this command in the directory of your zarf.yaml will result in the following output:

<iframe src="/docs/tutorials/prepare_find_images.html" height="220px" width="100%"></iframe>

From here you can copy the `images` key and array of images into the `wordpress` component we defined in our `zarf.yaml`

:::note

Due to the way some applications are deployed, Zarf might not be able to find all of the images in this way (particularly with operators).  For this you can look at the upstream charts or manifests and find them manually.

:::

:::tip

Zarf has more `prepare` commands you can learn about on the [prepare CLI docs page](../2-the-zarf-cli/100-cli-commands/zarf_prepare.md).

:::

### Setting up Variables

We now have a deployable package definition, but it is currently not very configurable and might not fit every environment we want to deploy it to.  If we deployed it as-is we would always have a Zarf Blog and a `zarf` user with an autogenerated password.

To resolve this, we can add configuration options with [Zarf Deploy-Time Variables](../../examples/variables/README.md#deploy-time-variables-and-constants).  For this package we will add a `variables` section to our `zarf.yaml` above `components` that will allow us to setup the user and the blog.

```yaml
variables:
    # The unique name of the variable corresponding to the ###ZARF_VAR_### template
  - name: WORDPRESS_USERNAME
    # A human-readable description of the variable shown during prompting
    description: The username that is used to login to the Wordpress admin account
    # A default value to take if --confirm is used or the user chooses the default prompt
    default: zarf
    # Whether to prompt for this value interactively if it is not --set on the CLI
    prompt: true
  - name: WORDPRESS_PASSWORD
    description: The password that is used to login to the Wordpress admin account
    prompt: true
    # Whether to treat this value as sensitive to keep it out of Zarf logs
    sensitive: true
  - name: WORDPRESS_EMAIL
    description: The email that is used for the Wordpress admin account
    default: hello@defenseunicorns.com
    prompt: true
  - name: WORDPRESS_FIRST_NAME
    description: The first name that is used for the Wordpress admin account
    default: Zarf
    prompt: true
  - name: WORDPRESS_LAST_NAME
    description: The last name that is used for the Wordpress admin account
    default: The Axolotl
    prompt: true
  - name: WORDPRESS_BLOG_NAME
    description: The blog name that is used for the Wordpress admin account
    default: The Zarf Blog
    prompt: true
```

To use these variables in our chart we must add their corresponding templates to our `wordpress-values.yaml` file.  Zarf can template chart values, manifests, included text files and more.

```yaml
wordpressUsername: ###ZARF_VAR_WORDPRESS_USERNAME###
wordpressPassword: ###ZARF_VAR_WORDPRESS_PASSWORD###
wordpressEmail: ###ZARF_VAR_WORDPRESS_EMAIL###
wordpressFirstName: ###ZARF_VAR_WORDPRESS_FIRST_NAME###
wordpressLastName: ###ZARF_VAR_WORDPRESS_LAST_NAME###
wordpressBlogName: ###ZARF_VAR_WORDPRESS_BLOG_NAME###
```

:::caution

When dealing with `sensitive` values in Zarf it is strongly recommended to not include them directly inside of a Zarf Package and to only define them at deploy-time.  You should also be aware of where you are using these values as they may be printed in `actions` you create or `files` that you place on disk.

:::

### Setting up a Zarf Connect Service

As-is, our package could be configured to interface with an ingress provider to provide access to our blog, but this may not be desired for every service, particularly those that provide a backend for other frontend services.  To help with debugging, Zarf allows you to specify Zarf Connect Services that will be displayed after package deployment to quickly connect into our deployed application.

For this package we will define two services, one for the blog and the other for the admin panel.  These are normal Kubernetes services with special labels and annotations that Zarf watches out for, and to defined them create a `connect-services.yaml` with the following contents:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: wordpress-connect-blog
  labels:
    # Enables "zarf connect wordpress-blog"
    zarf.dev/connect-name: wordpress-blog
  annotations:
    zarf.dev/connect-description: "The public facing Wordpress blog site"
spec:
  selector:
    app.kubernetes.io/instance: wordpress
    app.kubernetes.io/name: wordpress
  ports:
    - name: http
      port: 8080
      protocol: TCP
      targetPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: wordpress-connect-admin
  labels:
    # Enables "zarf connect wordpress-admin"
    zarf.dev/connect-name: wordpress-admin
  annotations:
    zarf.dev/connect-description: "The login page for the Wordpress admin panel"
    # Sets a URL-suffix to automatically navigate to in the browser
    zarf.dev/connect-url: "/wp-admin"
spec:
  selector:
    app.kubernetes.io/instance: wordpress
    app.kubernetes.io/name: wordpress
  ports:
    - name: http
      port: 8080
      protocol: TCP
      targetPort: 8080
```

To add this to our `zarf.yaml` we can simply specify it under our `wordpress` component using the `manifests` key:

```yaml
    manifests:
      - name: connect-services
        namespace: wordpress
        files:
          - connect-services.yaml
```

### Creating the Package

Once you have followed the above you should now have a `zarf.yaml` file that matches the one found on the [Wordpress example page](../../examples/wordpress/README.md).

Creating this package is as simple as running the `zarf package create` command with the directory containing our `zarf.yaml`.  Zarf will show us the `zarf.yaml` one last time asking if we would like to build the package, and will ask us for a maximum package size (useful if you need to split a package across multiple [Compact Discs](https://en.wikipedia.org/wiki/Compact_disc)).  Upon confirmation Zarf will pull down all of the resources and bundle them into a package tarball.

```bash
zarf package create .
```

When you execute the `zarf package create` command, Zarf will prompt you to confirm that you want to create the package by displaying the package definition and asking you to respond with either `y` or `n`.

<iframe src="/docs/tutorials/package_create_wordpress.html" height="500px" width="100%"></iframe>

:::tip

You can skip this confirmation by adding the `--confirm` flag when running the command. This will look like: `zarf package create . --confirm`

:::

After you confirm package creation, you have the option to specify a maximum file size for the package. To disable this feature, enter `0`.

<iframe src="/docs/tutorials/package_create_size.html" height="100px" width="100%"></iframe>

This will create a zarf package in the current directory with a package name that looks something like `zarf-package-wordpress-amd64-16.0.4.tar.zst`, although it might be slightly different depending on your system architecture.

:::tip

You can learn more about what is going on behind the scenes of this process on the [package create lifecycle page](../3-create-a-zarf-package/5-package-create-lifecycle.md), and can view other useful command flags like `--differential` and `--registry-override` on the [package create command flags page](../2-the-zarf-cli/100-cli-commands/zarf_package_create.md).

:::

Congratulations! You've built the Wordpress package. Now, you can learn how to [inspect the SBOMs](../4-deploy-a-zarf-package/4-view-sboms.md) or head straight to [deploying it](./2-deploying-zarf-packages.md)!

## Troubleshooting

### Unable to read zarf.yaml file

<iframe src="/docs/tutorials/package_create_error.html" height="120px" width="100%"></iframe>

:::info Remediation

If you receive this error, you may not be in the correct directory. Double-check where you are in your system and try again once you're in the correct directory with the zarf.yaml file that you're trying to build.

:::
