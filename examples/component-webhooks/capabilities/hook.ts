import { Capability, a, Log, K8s, kind } from "pepr";
import { DeployedPackage } from "./zarf-types";

/**
 *  The Webhook Capability is an example capability to demonstrate using webhooks to interact with Zarf package deployments.
 *  To test this capability you run `pepr dev`and then deploy a zarf package!
 */
export const Webhook = new Capability({
  name: "example-webhook",
  description:
    "A simple example capability to show how webhooks work with Zarf package deployments.",
  namespaces: ["zarf"],
});

const webhookName = "test-webhook";

const { When } = Webhook;

When(a.Secret)
  .IsCreatedOrUpdated()
  .InNamespace("zarf")
  .WithLabel("package-deploy-info")
  .Mutate(request => {
    const secret = request.Raw;
    let secretData: DeployedPackage;
    let secretString: string;
    let manuallyDecoded = false;

    // Pepr does not decode/encode non-ASCII characters in secret data: https://github.com/defenseunicorns/pepr/issues/219
    try {
      secretString = atob(secret.data.data);
      manuallyDecoded = true;
    } catch (err) {
      secretString = secret.data.data;
    }

    // Parse the secret object
    try {
      secretData = JSON.parse(secretString);
    } catch (err) {
      throw new Error(`Failed to parse the secret.data.data: ${err}`);
    }

    for (const deployedComponent of secretData?.deployedComponents ?? []) {
      if (deployedComponent.status === "Deploying") {
        Log.info(
          `The component ${deployedComponent.name} is currently deploying`,
        );

        const componentWebhook =
          secretData.componentWebhooks?.[deployedComponent?.name]?.[
            webhookName
          ];

        // Check if the component has a webhook running for the current package generation
        if (componentWebhook?.observedGeneration === secretData.generation) {
          Log.debug(
            `The component ${deployedComponent.name} has already had a webhook executed for it. Not executing another.`,
          );
        } else {
          // Seed the componentWebhooks map/object
          if (!secretData.componentWebhooks) {
            secretData.componentWebhooks = {};
          }

          // Update the secret noting that the webhook is running for this component
          secretData.componentWebhooks[deployedComponent.name] = {
            webhookName: {
              name: webhookName,
              status: "Running",
              observedGeneration: secretData.generation,
              waitDurationSeconds: 15,
            },
          };

          // Call an async function that simulates background processing and then updates the secret with the new status when it's complete
          sleepAndChangeStatus(secret.metadata.name, deployedComponent.name);
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
async function sleepAndChangeStatus(secretName: string, componentName: string) {
  await sleep(10);

  const ns = "zarf";

  let secret: a.Secret;

  // Fetch the package secret
  try {
    secret = await K8s(kind.Secret).InNamespace(ns).Get(secretName);
  } catch (err) {
    Log.error(
      `Error: Failed to get package secret '${secretName}' in namespace '${ns}': ${JSON.stringify(
        err,
      )}`,
    );
  }

  const secretString = atob(secret.data.data);
  const secretData: DeployedPackage = JSON.parse(secretString);

  // Update the webhook status if the observedGeneration matches
  const componentWebhook =
    secretData.componentWebhooks[componentName]?.[webhookName];

  if (componentWebhook?.observedGeneration === secretData.generation) {
    componentWebhook.status = "Succeeded";

    secretData.componentWebhooks[componentName][webhookName] = componentWebhook;
  }

  secret.data.data = btoa(JSON.stringify(secretData));

  // Update the status in the package secret
  // Use Server-Side force apply to forcefully take ownership of the package secret data.data field
  // Doing a Server-Side apply without the force option will result in a FieldManagerConflict error due to Zarf owning the object.
  try {
    await K8s(kind.Secret).Apply(
      {
        metadata: {
          name: secretName,
          namespace: ns,
        },
        data: {
          data: secret.data.data,
        },
      },
      { force: true },
    );
  } catch (err) {
    Log.error(
      `Error: Failed to update package secret '${secretName}' in namespace '${ns}': ${JSON.stringify(
        err,
      )}`,
    );
  }
}

function sleep(seconds: number) {
  return new Promise(resolve => setTimeout(resolve, seconds * 1000));
}
