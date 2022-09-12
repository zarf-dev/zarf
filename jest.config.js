// jest.config.js
export default {
	transform: {
		'^.+\\.ts$': 'ts-jest',
		'^.+\\.svelte$': [
			'svelte-jester',
			{
				preprocess: true
			}
		]
	},
	moduleFileExtensions: ['js', 'ts', 'svelte']
};
