export interface DeployedPackage {
  componentWebhooks?: { [key: string]: { [key: string]: Webhook } };
  deployedComponents: DeployedComponent[];
  generation: number;
  name: string;
}

export interface DeployedComponent {
  images: string[];
  name: string;
  observedGeneration: number;
  status: string;
}

export interface Webhook {
  name: string;
  observedGeneration: number;
  status: string;
  /**
   * The number of seconds that Zarf will wait for the webhook to run before timing out and returning an error.
   */
  waitDurationSeconds?: number;
}
