# Changelog

## [0.76.0](https://github.com/zarf-dev/zarf/compare/v0.75.1...v0.76.0) (2026-05-14)


### ⚠ BREAKING CHANGES

* **sign:** align signing and verification flags to cosign ([#4880](https://github.com/zarf-dev/zarf/issues/4880))

### Features

* add OCI support for Argo CD sources ([#4354](https://github.com/zarf-dev/zarf/issues/4354)) ([6b54e0a](https://github.com/zarf-dev/zarf/commit/6b54e0a40d3c75c0da5600fc116a6e2ecc611fdf))
* implement import logic for Zarf Values ([#4427](https://github.com/zarf-dev/zarf/issues/4427)) ([fde1211](https://github.com/zarf-dev/zarf/commit/fde1211e7ae2ec29dbc5c304d910949de3f5ba29))
* **sign:** align signing and verification flags to cosign ([#4880](https://github.com/zarf-dev/zarf/issues/4880)) ([04fb929](https://github.com/zarf-dev/zarf/commit/04fb9291bc958c7f0e2847f3a347873eab38bd94))
* **values:** package inspect values feature support ([#4867](https://github.com/zarf-dev/zarf/issues/4867)) ([2a3ef1b](https://github.com/zarf-dev/zarf/commit/2a3ef1b07f73e6c28c5351c256fa69663a0aa683))
* verify dynamic path fields in package config are clean ([#4883](https://github.com/zarf-dev/zarf/issues/4883)) ([579ec27](https://github.com/zarf-dev/zarf/commit/579ec276acdad294cc78f2149e341f497f7a1dac))


### Bug Fixes

* add cluster timeout cluster.New in dev deploy ([#4882](https://github.com/zarf-dev/zarf/issues/4882)) ([fe7fc39](https://github.com/zarf-dev/zarf/commit/fe7fc39eebba1eab0a0f9d3dbb77a4089f78569c))
* **deploy:** scope installedCharts by namespace override ([#4873](https://github.com/zarf-dev/zarf/issues/4873)) ([6c6ee8c](https://github.com/zarf-dev/zarf/commit/6c6ee8c5ca2702ada5d9ea9eebae3b968dca3da9))
* **init:** skip namespace agent labels when agent is disabled ([#4851](https://github.com/zarf-dev/zarf/issues/4851)) ([8ed2439](https://github.com/zarf-dev/zarf/commit/8ed2439272fa7bd5631b1d01b65f7eb96568614e))
* **variables:** file variable substitution alignment ([#4866](https://github.com/zarf-dev/zarf/issues/4866)) ([a16872c](https://github.com/zarf-dev/zarf/commit/a16872cd3d81496cabf831a897b4b60bd1509a72))

## [0.75.1](https://github.com/zarf-dev/zarf/compare/v0.75.0...v0.75.1) (2026-04-30)


### Features

* parse multi doc zarf.yaml files ([#4827](https://github.com/zarf-dev/zarf/issues/4827)) ([44ae0e2](https://github.com/zarf-dev/zarf/commit/44ae0e25503931e6825100a2a17eac192c2c497a))
* stop adding Zarf service default values to state when the service does not exist ([#4832](https://github.com/zarf-dev/zarf/issues/4832)) ([c4a06fb](https://github.com/zarf-dev/zarf/commit/c4a06fb5dc5f80c6577cddbea34189bfa54c591d))
* **verfication:** trusted root fetch command ([#4829](https://github.com/zarf-dev/zarf/issues/4829)) ([73825da](https://github.com/zarf-dev/zarf/commit/73825da520a53fa6c245bb8a6ee1138c1248e3b3))


### Bug Fixes

* **create:** built package path separators ([#4857](https://github.com/zarf-dev/zarf/issues/4857)) ([48574c2](https://github.com/zarf-dev/zarf/commit/48574c29288e740d5498fb469e77793e40bc1b46))
* ensure zarf say honors no-color ([#4850](https://github.com/zarf-dev/zarf/issues/4850)) ([f9748d5](https://github.com/zarf-dev/zarf/commit/f9748d5993d5c2eb9cf8d21c2db9e98ec9965ecd))
* **template:** add to dissallowed functions ([#4848](https://github.com/zarf-dev/zarf/issues/4848)) ([cedec4d](https://github.com/zarf-dev/zarf/commit/cedec4dcbae3e5cfd234c030ebd4c71aebfe7c8b))

## [0.75.0](https://github.com/zarf-dev/zarf/compare/v0.74.2...v0.75.0) (2026-04-16)


### ⚠ BREAKING CHANGES

* remove 0.27.0 layout shim ([#4826](https://github.com/zarf-dev/zarf/issues/4826))
* **deploy:** introduce connected mode ([#4685](https://github.com/zarf-dev/zarf/issues/4685))
* only pull required layers during OCI deploy ([#4699](https://github.com/zarf-dev/zarf/issues/4699))

### Features

* **deploy:** introduce connected mode ([#4685](https://github.com/zarf-dev/zarf/issues/4685)) ([bb4083d](https://github.com/zarf-dev/zarf/commit/bb4083d694ced66984ae0298cf8e2f7a5655dc3b))
* **init:** adopt arbitrarily named gitea PVC ([#4808](https://github.com/zarf-dev/zarf/issues/4808)) ([d874d5c](https://github.com/zarf-dev/zarf/commit/d874d5c1d777c3a8b472abbf3f78f1c0e0fe6c8b))
* **tls:** support for tls generation with duration ([#4769](https://github.com/zarf-dev/zarf/issues/4769)) ([1fd32d5](https://github.com/zarf-dev/zarf/commit/1fd32d56598767bc43211fe11fe44c132cccb829))


### Bug Fixes

* **cache:** support for default sdk cache path ([#4765](https://github.com/zarf-dev/zarf/issues/4765)) ([7b0e330](https://github.com/zarf-dev/zarf/commit/7b0e330042f883e3a775a58ffc4d057f4c0df28f))
* only pull required layers during OCI deploy ([#4699](https://github.com/zarf-dev/zarf/issues/4699)) ([05a3a36](https://github.com/zarf-dev/zarf/commit/05a3a3621055a9a6ac7251a07c8083a3a0f26472))
* remove 0.27.0 layout shim ([#4826](https://github.com/zarf-dev/zarf/issues/4826)) ([6106d17](https://github.com/zarf-dev/zarf/commit/6106d175111cfa964f10213bc37a02a00f5f386e))
* **verify:** deprecate PublicKeyPath in favor of VerifyBlobOptions ([#4782](https://github.com/zarf-dev/zarf/issues/4782)) ([2e8e58a](https://github.com/zarf-dev/zarf/commit/2e8e58a755d090c9ec6d09070c65ee1ee84bcfcc))

## [0.74.2](https://github.com/zarf-dev/zarf/compare/v0.74.1...v0.74.2) (2026-04-08)


### Bug Fixes

* git host matching ([#4801](https://github.com/zarf-dev/zarf/issues/4801)) ([20bead2](https://github.com/zarf-dev/zarf/commit/20bead2738e98835767020e5f94dc6c78b46b082))
* sanitize inspect output path ([#4793](https://github.com/zarf-dev/zarf/issues/4793)) ([abd00af](https://github.com/zarf-dev/zarf/commit/abd00afc217eeb2b99af8aa69dbde592f9b21621))

## [0.74.1](https://github.com/zarf-dev/zarf/compare/v0.74.0...v0.74.1) (2026-04-02)


### Features

* enable plugin support for vender-ed kubectl ([#4705](https://github.com/zarf-dev/zarf/issues/4705)) ([d812a6b](https://github.com/zarf-dev/zarf/commit/d812a6b5d28645cc5f9ac509dd38c96c8848c7d1))
* introduce page for schema on docs site ([#4732](https://github.com/zarf-dev/zarf/issues/4732)) ([1d193d0](https://github.com/zarf-dev/zarf/commit/1d193d00cab99c2f752ca54627692acc695550f6))
* **state:** deprecate "nodeport" in registry info in favor of "node" ([#4729](https://github.com/zarf-dev/zarf/issues/4729)) ([c8dd855](https://github.com/zarf-dev/zarf/commit/c8dd855cb3e3a80bdf24fac8f345ffc72c62be80))


### Bug Fixes

* **cache:** sbom cachepath existence ([#4762](https://github.com/zarf-dev/zarf/issues/4762)) ([8785473](https://github.com/zarf-dev/zarf/commit/8785473580b269a242955eb435973588d07ba6e1))
* set transport in `zarf tools registry catalog` when mtls is enabled ([#4728](https://github.com/zarf-dev/zarf/issues/4728)) ([b8e38ec](https://github.com/zarf-dev/zarf/commit/b8e38ecfc4f6beb5af90da448610709e75c1b62c))
* values with zarf dev find-images ([#4734](https://github.com/zarf-dev/zarf/issues/4734)) ([78b7202](https://github.com/zarf-dev/zarf/commit/78b7202b41d909b520d606973708aaf033c6006b))

## [0.74.0](https://github.com/zarf-dev/zarf/compare/v0.73.1...v0.74.0) (2026-03-19)


### ⚠ BREAKING CHANGES

* upgrade to Helm 4 ([#4350](https://github.com/zarf-dev/zarf/issues/4350))
* **deploy:** override actions wait commands ([#4531](https://github.com/zarf-dev/zarf/issues/4531))

### Features

* add retries on create operations ([#4664](https://github.com/zarf-dev/zarf/issues/4664)) ([23afb84](https://github.com/zarf-dev/zarf/commit/23afb84546c9c51a8a6ee7186f693f6be79fe5e8))
* **connect:** create zarf connect resource sub-command ([#4683](https://github.com/zarf-dev/zarf/issues/4683)) ([e4f9299](https://github.com/zarf-dev/zarf/commit/e4f9299ec16e572631e2e39180a99709649d1d9f))
* **init:** clarify --registry-secret or --registry-url ([#4694](https://github.com/zarf-dev/zarf/issues/4694)) ([3ea9a4d](https://github.com/zarf-dev/zarf/commit/3ea9a4df8099f922713bcdd52e84900bc4a045a3))
* **init:** enable switching between nodeport and proxy mode ([#4608](https://github.com/zarf-dev/zarf/issues/4608)) ([bb5d1df](https://github.com/zarf-dev/zarf/commit/bb5d1dff046fa705d608ea8b7e188beddd137b29))
* **publish:** support for tag specification ([#4641](https://github.com/zarf-dev/zarf/issues/4641)) ([9cf8912](https://github.com/zarf-dev/zarf/commit/9cf891257763973e79618428ea08e92d5139ec80))
* **state:** remove architecture field ([#4701](https://github.com/zarf-dev/zarf/issues/4701)) ([910c646](https://github.com/zarf-dev/zarf/commit/910c64601a48e951b7c5316fecca6ab3df874349))
* stop managing scale down policy in CLI ([#4725](https://github.com/zarf-dev/zarf/issues/4725)) ([d305f82](https://github.com/zarf-dev/zarf/commit/d305f8263fe1b29ad43b828a0e37c5be4f5424c9))
* update kubectl vender logic ([#4676](https://github.com/zarf-dev/zarf/issues/4676)) ([0847c52](https://github.com/zarf-dev/zarf/commit/0847c525cab3d589ec8ed2522d935afa8a7df212))
* upgrade to Helm 4 ([#4350](https://github.com/zarf-dev/zarf/issues/4350)) ([505d1df](https://github.com/zarf-dev/zarf/commit/505d1df856a90f35f300a422308311a65a208993))
* use legacy Helm wait + reconciliation Healthchecks ([#4720](https://github.com/zarf-dev/zarf/issues/4720)) ([fde9d53](https://github.com/zarf-dev/zarf/commit/fde9d53cf89e37c94987dcfe0886e4f58931a7dc))
* use Zarf Package Config as image config ([#4675](https://github.com/zarf-dev/zarf/issues/4675)) ([e9262d4](https://github.com/zarf-dev/zarf/commit/e9262d412aa04ca20400d6197c8b9c04e0028eb3))


### Bug Fixes

* **agent:** support create idempotency for mutation operations ([#4691](https://github.com/zarf-dev/zarf/issues/4691)) ([d0cdef9](https://github.com/zarf-dev/zarf/commit/d0cdef98b3cd9c9da2a656bf8f02dadc8f9ac0cd))
* close chunk file descriptors per iteration in `SplitFile` ([#4656](https://github.com/zarf-dev/zarf/issues/4656)) ([a9d1700](https://github.com/zarf-dev/zarf/commit/a9d1700c89f06cfd8669484d8d083014bdf16069))
* close leaked file handles in `pull_test.go` HTTP handlers ([#4657](https://github.com/zarf-dev/zarf/issues/4657)) ([0ef41d7](https://github.com/zarf-dev/zarf/commit/0ef41d7ac9f81c0b495a0e35c8a82155d43f7fca))
* **deploy:** override actions wait commands ([#4531](https://github.com/zarf-dev/zarf/issues/4531)) ([39fd337](https://github.com/zarf-dev/zarf/commit/39fd337949547078d14509b7d6343a43fb3c65f1))
* set field manager once during pre-run to avoid data race ([#4707](https://github.com/zarf-dev/zarf/issues/4707)) ([88f60fa](https://github.com/zarf-dev/zarf/commit/88f60fab317177465801dc023d3efc0a855e0083))

## [0.73.1](https://github.com/zarf-dev/zarf/compare/v0.73.0...v0.73.1) (2026-03-03)


### Bug Fixes

* **archive:** update to use os.root API ([#4674](https://github.com/zarf-dev/zarf/issues/4674)) ([93f9c33](https://github.com/zarf-dev/zarf/commit/93f9c33a9d4724ea3fa51d09a69e8b7f8525dc57))
* buffer `errChan` in `Tunnel.establish` to prevent goroutine leak ([#4653](https://github.com/zarf-dev/zarf/issues/4653)) ([f087c17](https://github.com/zarf-dev/zarf/commit/f087c17897816b5e86f560cdc0b31de44f8eb1ae))
* check `svc.Spec.Ports` bounds before indexing in tunnel code ([#4654](https://github.com/zarf-dev/zarf/issues/4654)) ([1d017f4](https://github.com/zarf-dev/zarf/commit/1d017f4ddc2324f768d42c86c6eb047d8294071e))
* preserve error chains by using `%w` instead of `%s` ([#4658](https://github.com/zarf-dev/zarf/issues/4658)) ([3a4875e](https://github.com/zarf-dev/zarf/commit/3a4875e8187a510ecb3fbc27455732b2a9b64c95))
* prevent panic on double call to `Tracker.StopReporting` ([#4655](https://github.com/zarf-dev/zarf/issues/4655)) ([2d19e74](https://github.com/zarf-dev/zarf/commit/2d19e7452c6aaceeeec3b901169f753e47f11078))
* return the correct error on io.CopyN failure ([#4652](https://github.com/zarf-dev/zarf/issues/4652)) ([c69273c](https://github.com/zarf-dev/zarf/commit/c69273c5ed0afea1793b03127c5ac15c07d932e9))

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
