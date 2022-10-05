import { DeployingComponents } from '$lib/api';
import type { DeployedComponent, ZarfComponent } from '$lib/api-types';
import type { StepProps } from '@defense-unicorns/unicorn-ui/Stepper/stepper.types';

const POLL_TIME_MS = 2000;

export type ComponentStepMap = Map<string, StepProps>;

export function createComponentStepMap(
	allComponents: ZarfComponent[],
	deployComponentIdx: number[]
): ComponentStepMap {
	let deployingComponentMap: ComponentStepMap = new Map();
	deployComponentIdx.forEach((componentIndex: number, index: number) => {
		let component = allComponents[componentIndex];
		deployingComponentMap.set(component.name, {
			title: `Deploy ${component.name}`,
			variant: 'primary',
			iconContent: index.toString(),
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

export async function getDeployedComponents(components: ComponentStepMap): Promise<StepProps[]> {
	(await DeployingComponents.list()).forEach((component: DeployedComponent) => {
		const componentStep = components.get(component.name);
		if (componentStep) {
			components.set(component.name, setStepSuccessful(componentStep));
		}
	});
	return getComponentStepMapComponents(components);
}
