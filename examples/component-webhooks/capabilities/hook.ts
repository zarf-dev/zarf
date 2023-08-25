import {
  Capability,
  a,
  Log,
  k8s,
} from "pepr";


/**
 *  The Webhook Capability is an example capability to demonstrate using webhooks to interact with Zarf package deployments.
 *  To test this capability you run `pepr dev`and then deploy a zarf package!
 */
export const Webhook = new Capability({
  name: "example-webhook",
  description: "A simple example capability to show how webhooks work with Zarf package deployments.",
  namespaces: ["zarf"],
});

const {When} = Webhook;

When(a.Secret)
  .IsCreatedOrUpdated()
  .InNamespace("zarf")
  .WithLabel("package-deploy-info")
  .Then(request => {
    const secret = request.Raw;
    let secretData;
    let secretString: string;
    let manuallyDecoded = false;

    // Pepr does not decode/encode non-ASCII characters in secret data: https://github.com/defenseunicorns/pepr/issues/219
    try {
      secretString = atob(secret.data.data);
      manuallyDecoded = true;
    } catch (error) {
      secretString = secret.data.data;
    }

    // Parse the secret object
    try {
      secretData = JSON.parse(secretString);
    } catch (error) {
      Log.error("failed to parse the secret.data.data: " + error);
      return;
    }

    for (const component of secretData?.deployedComponents ?? []) {
      if (component.status === "Deploying") {
        Log.debug(`The component ${component.name} is deploying`);

        const componentWebhook = secretData.componentWebhooks?.[component?.name]?.["test-webhook"];

        // Check if the component has a webhook running for the current package generation
        if (componentWebhook?.observedGeneration === secretData.generation) {
          Log.debug(`The component ${component.name} has already had a webhook executed for it. Not executing another.`);
        } else {
          // Seed the componentWebhooks map/object
          if (!secretData.componentWebhooks) {
            secretData.componentWebhooks = {};
          }

          // Update the secret noting that the webhook is running for this component
          secretData.componentWebhooks[component.name] = {
            "test-webhook": {
              "name": "test-webhook",
              "status": "Running",
              "observedGeneration": secretData.generation,
            },
          };

          // Call an async function that simulates background processing and then updates the secret with the new status when it's complete
          sleepAndChangeStatus(secret.metadata.name, component.name);
        }
      }
    }

    if (manuallyDecoded === true) {
      secret.data.data = btoa(JSON.stringify(secretData));
    } else {
      secret.data.data = JSON.stringify(secretData);
    }
});

// sleepAndChangeStatus sleeps for the specified duration and changes the status of the 'test-webhook' to 'Succeeded'.
async function sleepAndChangeStatus(secretName : string, componentName : string)  {
  await sleep(10)

  // Configure the k8s api client
  const kc = new k8s.KubeConfig();
  kc.loadFromDefault();
  const k8sCoreApi = kc.makeApiClient(k8s.CoreV1Api);

  try {
    const response = await k8sCoreApi.readNamespacedSecret(secretName, "zarf");
    const v1Secret = response.body;
    
    const secretString = atob(v1Secret.data.data);
    const secretData = JSON.parse(secretString);
  
    // Update the webhook status if the observedGeneration matches
    const componentWebhook = secretData.componentWebhooks[componentName]?.["test-webhook"];
    if (componentWebhook?.observedGeneration === secretData.generation) {
      componentWebhook.status = "Succeeded";
      secretData.componentWebhooks[componentName]["test-webhook"] = componentWebhook;
    }
  
    v1Secret.data.data = btoa(JSON.stringify(secretData));
  
    // Patch the secret back to the cluster
    await k8sCoreApi.patchNamespacedSecret(
      secretName,
      "zarf",
      v1Secret,
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
      { headers: { 'Content-Type': 'application/strategic-merge-patch+json' } }
    );
  } catch (err) {
    Log.error(`Unable to update the package secret: ${JSON.stringify(err)}`);
    return err;
  }
  
}

function sleep(seconds: number) {
  return new Promise(resolve => setTimeout(resolve, seconds * 1000));
}
