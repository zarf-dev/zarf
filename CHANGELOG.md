# Changelog

## [0.73.0](https://github.com/zarf-dev/zarf/compare/v0.72.0...v0.73.0) (2026-02-20)


### ⚠ BREAKING CHANGES

* **SDK:** avoid os exit in cmd ([#4615](https://github.com/zarf-dev/zarf/issues/4615))

### Features

* **SDK:** avoid os exit in cmd ([#4615](https://github.com/zarf-dev/zarf/issues/4615)) ([7f67816](https://github.com/zarf-dev/zarf/commit/7f67816c654ef22d94b575857c2c4a7c2c59e640))
* split wait-for command ([#4614](https://github.com/zarf-dev/zarf/issues/4614)) ([3340ead](https://github.com/zarf-dev/zarf/commit/3340ead7248ac411bbaf91921070b583198aa98f))


### Bug Fixes

* **wait:** properly resolve kind when group conflicts between resources ([#4628](https://github.com/zarf-dev/zarf/issues/4628)) ([db3cd9d](https://github.com/zarf-dev/zarf/commit/db3cd9d5e31aae78ed99f8f3e2bdfbe46e0a4e13))

## [0.72.0](https://github.com/zarf-dev/zarf/compare/v0.71.1...v0.72.0) (2026-02-19)


### ⚠ BREAKING CHANGES

* **bundle:** bundle feature flag and version requirement ([#4600](https://github.com/zarf-dev/zarf/issues/4600))

### Features

* add ability to supply custom init package ([#4562](https://github.com/zarf-dev/zarf/issues/4562)) ([f09b126](https://github.com/zarf-dev/zarf/commit/f09b1269531e17e90f492d07ebaa29eca0e46081))


### Bug Fixes

* **bundle:** bundle feature flag and version requirement ([#4600](https://github.com/zarf-dev/zarf/issues/4600)) ([24f2738](https://github.com/zarf-dev/zarf/commit/24f2738c54d322dcff851e54c59a31e8e72cd831))
* **make:** always run unit tests with -race flag ([#4610](https://github.com/zarf-dev/zarf/issues/4610)) ([76950b3](https://github.com/zarf-dev/zarf/commit/76950b34c37109f70cfbb9b29f605d8cf9467e53))
* **skeleton:** better error for missing skeleton ([#4611](https://github.com/zarf-dev/zarf/issues/4611)) ([25b3c78](https://github.com/zarf-dev/zarf/commit/25b3c7804830a4fecc9dd2466a33e7e7f85a3a9f))
* template variables and values in `.wait` actions ([#4604](https://github.com/zarf-dev/zarf/issues/4604)) ([bfc0582](https://github.com/zarf-dev/zarf/commit/bfc05823fc29ac28b931ce5a30ce63769a8fc8e5))
* use cli tmpdir arg for image unpacks ([#4618](https://github.com/zarf-dev/zarf/issues/4618)) ([ea6dc0f](https://github.com/zarf-dev/zarf/commit/ea6dc0fdc3c170c0032fc4963d9c558462f52094))
* **wait:** ensure cluster is connectable in loop ([#4616](https://github.com/zarf-dev/zarf/issues/4616)) ([ade37d0](https://github.com/zarf-dev/zarf/commit/ade37d0d1215d523d79c7cb84d220c0b61d754c3))


### Refactoring

* **wait:** avoid shelling out to kubectl during wait ([#4567](https://github.com/zarf-dev/zarf/issues/4567)) ([3660ece](https://github.com/zarf-dev/zarf/commit/3660ece20af29749fe3066f8f7a451132f359734))

## [0.71.1](https://github.com/zarf-dev/zarf/compare/v0.71.0...v0.71.1) (2026-02-06)


### Bug Fixes

* **actions:** shell quote action wait bug ([#4588](https://github.com/zarf-dev/zarf/issues/4588)) ([331f46e](https://github.com/zarf-dev/zarf/commit/331f46e4ef28b52a4dcc775e9376c633a1eafea8))

## [0.71.0](https://github.com/zarf-dev/zarf/compare/v0.70.1...v0.71.0) (2026-02-04)


### ⚠ BREAKING CHANGES

* **registry-proxy:** built-in mtls support ([#4162](https://github.com/zarf-dev/zarf/issues/4162))
* **wait:** create wait package and call it directly within actions ([#4549](https://github.com/zarf-dev/zarf/issues/4549))
* remove global plainHTTP and insecureSkipTLSVerify in favor of optional parameters ([#4561](https://github.com/zarf-dev/zarf/issues/4561))
* move v1alpha1 validation logic to it's own package ([#4544](https://github.com/zarf-dev/zarf/issues/4544))
* remove direct usage of parent command `zarf package inspect` ([#4548](https://github.com/zarf-dev/zarf/issues/4548)) ([1904293](https://github.com/zarf-dev/zarf/commit/19042935276f23f9d50101363008ebce987b7e11))

### Features

* add nodeSelector to zarf agent and registry charts ([#4565](https://github.com/zarf-dev/zarf/issues/4565)) ([a23e909](https://github.com/zarf-dev/zarf/commit/a23e909cf4a2029a344128ef9a8951c716ce9e3d))
* error early during healthchecks when status is terminal ([#4547](https://github.com/zarf-dev/zarf/issues/4547)) ([eb54546](https://github.com/zarf-dev/zarf/commit/eb5454614545b3710a5c21503d72e14b13943d42))
* **prune:** allow for pruning to continue on manifest unknown ([#4566](https://github.com/zarf-dev/zarf/issues/4566)) ([6814ead](https://github.com/zarf-dev/zarf/commit/6814eadbad76a9c5e5e0e14ebd7b77714596e80d))
* **registry-proxy:** built-in mtls support ([#4162](https://github.com/zarf-dev/zarf/issues/4162)) ([b493381](https://github.com/zarf-dev/zarf/commit/b493381ee5deda31336e89f187c8ece51dd522fe))
* **release:** add release please workflow and docs ([#4558](https://github.com/zarf-dev/zarf/issues/4558)) ([b4cb102](https://github.com/zarf-dev/zarf/commit/b4cb1027e953fc1787c0d124fd5c69105c3ef3a1))
* remove direct usage of parent command `zarf package inspect` ([#4548](https://github.com/zarf-dev/zarf/issues/4548)) ([1904293](https://github.com/zarf-dev/zarf/commit/19042935276f23f9d50101363008ebce987b7e11))
* remove global plainHTTP and insecureSkipTLSVerify in favor of optional parameters ([#4561](https://github.com/zarf-dev/zarf/issues/4561)) ([3eed404](https://github.com/zarf-dev/zarf/commit/3eed404fab86dc0d6fcd6f50de48f12d1dfa71d8))
* **sign:** implement support for sigstore bundle format ([#4519](https://github.com/zarf-dev/zarf/issues/4519)) ([9c3d446](https://github.com/zarf-dev/zarf/commit/9c3d446509767a823a03ed69cf8366242ac4db9e))
* **values:** support for chart values merge ([#4581](https://github.com/zarf-dev/zarf/issues/4581)) ([81df552](https://github.com/zarf-dev/zarf/commit/81df552d38990a9fe7d0ccb0bc0a33ae173403d6))


### Bug Fixes

* **helm:** preserve block scalar semantics ([#4541](https://github.com/zarf-dev/zarf/issues/4541)) ([8655c1c](https://github.com/zarf-dev/zarf/commit/8655c1c1e846de93bff378d6de19de20721bfbff))


### Refactoring

* move v1alpha1 validation logic to it's own package ([#4544](https://github.com/zarf-dev/zarf/issues/4544)) ([502a6be](https://github.com/zarf-dev/zarf/commit/502a6be130ec2d36bf0d67b4a458117e3ac47c7c))
* **wait:** create wait package and call it directly within actions ([#4549](https://github.com/zarf-dev/zarf/issues/4549)) ([2498e1c](https://github.com/zarf-dev/zarf/commit/2498e1cb96fdfb247aaef34f87bdbdf2b98c975f))
