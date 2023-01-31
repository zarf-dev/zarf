// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import { DeployingComponents } from '$lib/api';
import type { DeployedComponent, ZarfComponent } from '$lib/api-types';
import type { StepProps } from '@defense-unicorns/unicorn-ui/Stepper/stepper.types';

export type ComponentStepMap = Map<string, StepProps>;

// Returns a map of the component to deploy name as key and StepProps as value.
export function createComponentStepMap(
	allComponents: ZarfComponent[],
	deployComponentIdx: number[]
): ComponentStepMap {
	const deployingComponentMap: ComponentStepMap = new Map();

	deployComponentIdx.forEach((componentIndex: number, index: number) => {
		const component = allComponents[componentIndex];

		deployingComponentMap.set(component.name, {
			title: `Deploy ${component.name}`,
			iconContent: `${index + 1}`,
			disabled: true
		});
	});
	return deployingComponentMap;
}

export function getComponentStepMapComponents(componentSteps: ComponentStepMap): StepProps[] {
	return Array.from(componentSteps.values());
}

export function setStepSuccessful(step: StepProps): StepProps {
	return { ...step, variant: 'success', disabled: false, iconContent: undefined };
}

export function setStepError(step: StepProps): StepProps {
	return { ...step, variant: 'error', disabled: false, iconContent: undefined };
}

// On deploy success: sets all remaining steps state to success
// On deploy failure: sets all remaining steps to error
export function finalizeStepState(steps: StepProps[], success: boolean): StepProps[] {
	return steps.map((step: StepProps): StepProps => {
		let stepState = step;
		if (success) {
			stepState = setStepSuccessful(step);
		} else {
			if (step.variant !== 'success') {
				stepState = setStepError(step);
			}
		}
		return stepState;
	});
}

// Retrieves the components that (as far as we know) have successfully deployed.
export async function getDeployedComponents(components: ComponentStepMap): Promise<StepProps[]> {
	(await DeployingComponents.list()).forEach((component: DeployedComponent) => {
		const componentStep = components.get(component.name);
		if (componentStep) {
			components.set(component.name, setStepSuccessful(componentStep));
		}
	});
	return getComponentStepMapComponents(components);
}

export function getDialogContent(success: boolean): { topLine: string; bottomLine: string } {
	return success
		? {
				topLine: 'Package successfully deployed',
				bottomLine: 'You will be automatically redirected to the deployment details page.'
		  }
		: {
				topLine: 'Package failed to deploy',
				bottomLine: 'You will be automatically redirected to the home page.'
		  };
}
