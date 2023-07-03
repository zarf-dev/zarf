// Create a custom loader to pull in YAML files into the docusaurus webpack build
module.exports = function (context, options) {
    return {
        name: 'custom-loaders',
        configureWebpack(config, isServer) {
            return {
                module: {
                    rules: [
                        {
                            // Look for all require("*.yaml") files
                            test: /\.yaml/,
                            // Set this as an asset so it is pulled in as-is without compression
                            type: 'asset/resource',
                            // Generate a filename to place the example next to the generated index.html file
                            // (note this adds a fake "build" directory otherwise the examples get placed one directory too high)
                            // (it also adds a hash since there can be times when the same file is included twice and it needs to be different)
                            generator: {
                                filename: 'build/[file].[hash]'
                            }
                        },
                    ],
                },
            };
        },
    };
};
