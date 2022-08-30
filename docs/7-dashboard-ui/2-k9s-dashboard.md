# K9s

Zarf vendors in [K9s](https://k9scli.io/), a terminal based UI to interact with your Kubernetes cluster. K9s is not necessary to deploy, manage, or operate Zarf or its deployed packages, but it is a great tool to use when you want to interact with your cluster. Since Zarf vendors in this tool, you don't have to worry about additional dependencies or trying to install it yourself!


## Using the k9s Dashboard

All you need to use the k9s dashboard is to:
1. Have access to a running cluster kubecontext
1. Have a zarf binary installed

<br />
Using the k9s Dashboard is as simple as using a single command!

```bash
zarf tools k9s
```
<br />

**Example k9s Dashboard**
![k9s dashboard](../.images/dashboard/k9s_dashboard_example.png)

More instructions on how to use k9s can be found on their [documentation site](https://k9scli.io/topics/commands/).
