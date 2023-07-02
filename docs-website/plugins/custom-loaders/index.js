module.exports = function (context, options) {
    return {
        name: 'custom-loaders',
        configureWebpack(config, isServer) {
            return {
                module: {
                    rules: [
                        {
                            test: /\.yaml/,
                            type: 'asset/resource',
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
