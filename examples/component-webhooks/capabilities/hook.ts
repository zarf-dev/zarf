import {
  Capability,
  a,
  Log,
  k8s,
} from "pepr";


/**
 *  The  Capability is an example capability to demonstrate using webhooks to interact with Zarf package deployments.
 *  To test this capability you run `pepr dev`and then deploy a zarf package!
 */
export const Webhook = new Capability({
  name: "example-webhook",
  description: "A simple example capability to show how webhooks work with Zarf package deployments.",
  namespaces: ["zarf"],
});

const {When} = Webhook;

When(a.Secret).IsCreatedOrUpdated().InNamespace("zarf").WithLabel("package-deploy-info").Then( request => {
  const secret = request.Raw;
  let secretData;
  let secretString: string;
  let manuallyDecoded = false;

  // Pepr tries to decode the secret data, but it doesn't always do the job correctly, so attempt to do that ourselves
  try {
    secretString = atob(secret.data.data)
    manuallyDecoded = true
  } catch (error){
    secretString = secret.data.data
  }


  // Parse the secret object
  try {
    secretData = JSON.parse(secretString)
  } catch (error) {
    Log.error("failed to parse the secret.data.data: " + error)
    return
  }



  // Get the deployedComponents from the secret
  if (secretData) {
    if (secretData.deployedComponents) {

      // Loop through the deployedComponents and find any that are deploying
      secretData.deployedComponents.forEach( (component: any) => {
        if (component.status === "Deploying") {
          Log.debug("The component " + component.name + " is deploying")

          // Check if the component has a webhook running for the current package generation
          if (secretData.componentWebhooks &&
              secretData.componentWebhooks[component.name] &&
              secretData.componentWebhooks[component.name]["test-webhook"] &&
              secretData.componentWebhooks[component.name]["test-webhook"].observedGeneration === secretData.generation) {
            Log.debug("The component " + component.name + " has already had a webhook executed for it. Not executing another.")
          } else {

            // Seed the componentWebhooks map/object
            if (!secretData.componentWebhooks){
              secretData.componentWebhooks = {}
            }

            // Update the secret with information denoting that a test-webhook is running for this component
            secretData.componentWebhooks[component.name] = {"test-webhook": {"name": "test-webhook", "status": "Running", "observedGeneration": secretData.generation}}

            // Call an async function that simulates background processing and then updates the secret with the new status when it's complete
            sleepAndChangeStatus(secret.metadata.name, component.name)
          }
        }
      })
    }
  }

  // If Pepr didn't decode the secret data, it won't encode it for us either. This is a flakey known issue within Pepr.
  if (manuallyDecoded === true) {
    secret.data.data = btoa(JSON.stringify(secretData))
  } else {
    secret.data.data = JSON.stringify(secretData)
  }
});

// sleepAndChangeStatus sleeps for 30 seconds then changes the status of the 'test-webhook' to 'Succeeded'.
async function sleepAndChangeStatus(secretName : string, componentName : string)  {

  // Perform a sleep for 30 seconds that simulates background processing
  await sleep(30)

  // Configure the k8s api client
  const kc = new k8s.KubeConfig();
  kc.loadFromDefault();
  const k8sCoreApi = kc.makeApiClient(k8s.CoreV1Api);


  // Update the secret with the new status
  return await k8sCoreApi.readNamespacedSecret(secretName, "zarf").then( async response => {

      // Get the current secret from the response body
      let v1Secret = response.body
      let secretString = atob(v1Secret.data.data)
      let secretData = JSON.parse(secretString)

      // Change the status of the 'test-webhook' to 'Succeeded'
      let componentWebhook = secretData.componentWebhooks[componentName]["test-webhook"]
      componentWebhook.status = "Succeeded"
      secretData.componentWebhooks[componentName]["test-webhook"] = componentWebhook

      // Re-encode the secret and save it back to the cluster
      v1Secret.data.data = btoa(JSON.stringify(secretData))


      return await k8sCoreApi.patchNamespacedSecret(secretName, "zarf", v1Secret, undefined, undefined, undefined, undefined, undefined, {headers: {'Content-Type': 'application/strategic-merge-patch+json'}})
  }).catch( err => {
      Log.error(`unable to update the package secret: ${JSON.stringify(err)}`)
      return err
  })
}

function sleep(seconds: number) {
  return new Promise(resolve => setTimeout(resolve, seconds * 1000));
}
