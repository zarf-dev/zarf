// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import { Packages } from '$lib/api';
import type { ZarfComponent } from '$lib/api-types';
import type { StepProps } from '@defense-unicorns/unicorn-ui/Stepper/stepper.types';

export type ComponentStepMap = Map<string, StepProps>;
export type DeployedSteps = {steps: StepProps[], activeStep: number};

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
			disabled: index > 0,
		});
	});
	return deployingComponentMap;
}

export function getComponentStepMapComponents(componentSteps: ComponentStepMap): StepProps[] {
	return Array.from(componentSteps.values());
}

export function setStepActive(step: StepProps): StepProps {
	return { ...step, disabled: false };
}

export function setStepSuccessful(step: StepProps): StepProps {
	return { ...step, variant: 'success', disabled: false, iconContent: '' };
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
export async function getDeployedComponents(pkgName: string, components: ComponentStepMap): Promise<DeployedSteps> {
	const oldComponents = getComponentStepMapComponents(components);
	const deployingComponents = await Packages.deployingComponents.list(pkgName);
	const activeStep = deployingComponents.length;
	 const steps =  oldComponents.map((component: StepProps, idx: number) => {
		if (deployingComponents && deployingComponents[idx] && component.iconContent) {
			return setStepSuccessful(component);
		} else if (idx === deployingComponents.length) {
			return setStepActive(component);
		}
		return component;
	});
	return {steps, activeStep};
}

export function getDialogContent(success: boolean): { topLine: string; bottomLine: string } {
	return success
		? {
				topLine: 'Package successfully deployed',
				bottomLine: 'You will be automatically redirected to the home page.',
		  }
		: {
				topLine: 'Package failed to deploy',
				bottomLine: 'You will be automatically redirected to the home page.',
		  };
}
