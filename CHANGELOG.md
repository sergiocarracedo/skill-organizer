# Changelog

All notable changes to this project will be documented in this file.

## [1.3.0](https://github.com/sergiocarracedo/skill-organizer/compare/v1.2.0...v1.3.0) (2026-06-15)


### Features

* **release:** preflight-check New Relic vars before goreleaser ([7abd337](https://github.com/sergiocarracedo/skill-organizer/commit/7abd3370e37cd06ff5ee80d1e1c85e0e6ef6c779))
* **telemetry:** warn in status when binary is missing New Relic credentials ([967989e](https://github.com/sergiocarracedo/skill-organizer/commit/967989e4e6d743ac85f27609f7d3fe8ec01a5017))


### Bug Fixes

* **release:** read New Relic credentials from vars, not secrets ([1d217c0](https://github.com/sergiocarracedo/skill-organizer/commit/1d217c0c001eb43c851f00612414ab4cd7434749))

## [1.2.0](https://github.com/sergiocarracedo/skill-organizer/compare/v1.1.0...v1.2.0) (2026-06-15)


### Features

* **web:** move check-security to Smart features, add malicious skill inspection tab ([d9c81d4](https://github.com/sergiocarracedo/skill-organizer/commit/d9c81d42e49f6685382aaa79cbe58b13926d0125))

## [1.1.0](https://github.com/sergiocarracedo/skill-organizer/compare/v1.0.3...v1.1.0) (2026-06-15)


### Features

* **02-01:** add --allow-overlap flag and non-zero exit to check-overlap ([12b4b70](https://github.com/sergiocarracedo/skill-organizer/commit/12b4b70a3afa0815d2356f73a6da6ea04ba335ae))
* **03-01:** add oklog/ulid/v2 dependency for telemetry event IDs ([8cd079e](https://github.com/sergiocarracedo/skill-organizer/commit/8cd079e012e65157ddf5d8876f6ad73d873f31af))
* **03-01:** add telemetry Event struct with schema validation ([e94f861](https://github.com/sergiocarracedo/skill-organizer/commit/e94f8618035c651f16ee341592e336c3596bd699))
* **03-01:** add telemetry Identity type with LoadOrCreate and RotateHostID ([2f13ed8](https://github.com/sergiocarracedo/skill-organizer/commit/2f13ed832965d17c86316ae41d9c938a88b565b8))
* **03-01:** add telemetry Recorder interface, NoopRecorder, and factory ([1410f50](https://github.com/sergiocarracedo/skill-organizer/commit/1410f502cb57e0972681942be8d87cf4f19cb458))
* **03-02:** add Buffer JSONL spool with O_APPEND writes and 1MB FIFO eviction ([5cfd992](https://github.com/sergiocarracedo/skill-organizer/commit/5cfd992e5f5a2089fece5cd4241e7656dc4bbdf1))
* **03-02:** add cmd/telemetry.go with enable|disable|status|rotate-host-id ([365af29](https://github.com/sergiocarracedo/skill-organizer/commit/365af29911751ab0ea92b5a04f2c6c4e28d13cc0))
* **03-02:** add HTTPRecorder, RecorderConfig, SetDefaultFactory to recorder.go ([49a6721](https://github.com/sergiocarracedo/skill-organizer/commit/49a67213f0aab0ce5364151fd496fc50bba9f833))
* **03-02:** add prompt.go with TTY-gated FirstRunPrompt and func-var seam ([a270f45](https://github.com/sergiocarracedo/skill-organizer/commit/a270f45a9a81de4d593fc8aef19d2a1c9e985167))
* **03-02:** add telemetry.go with Service, RecordEvent, DrainBuffer, and alias normalization ([1481e5d](https://github.com/sergiocarracedo/skill-organizer/commit/1481e5d2d47868296e60ed4117472c63cf7304ee))
* **03-02:** add TelemetryConfig struct and Load/Save helpers ([36822e2](https://github.com/sergiocarracedo/skill-organizer/commit/36822e2bebd8f274d49e28074c4a1bba01eb5b3f))
* **03-02:** register telemetry subcommand in root.go + add cmd/telemetry_test.go ([52f2c4c](https://github.com/sergiocarracedo/skill-organizer/commit/52f2c4c54c7913b6b5e852aae3647b95890bab99))
* **03-02:** wire root.go PersistentPreRun/PostRun + --telemetry-endpoint flag ([4638ca9](https://github.com/sergiocarracedo/skill-organizer/commit/4638ca9ce8e1867ca4092fe09db5ccc9b6105dc6))
* **05-02:** append telemetry disable off-ramp to first-run prompt copy ([faf0286](https://github.com/sergiocarracedo/skill-organizer/commit/faf0286acf1a2cb1654c360b79cc99d3cd9dd1a1))
* **06-02:** add --model flag to check-overlap command ([304a296](https://github.com/sergiocarracedo/skill-organizer/commit/304a29618a118dddf6b4c446163e77608dfa390f))
* **06-02:** add --model flag to check-security command ([3b3e090](https://github.com/sergiocarracedo/skill-organizer/commit/3b3e090543fddbef4f758644f0db911155cd8d02))
* **06-02:** add model selection to chooseAgentToolImpl ([0d70142](https://github.com/sergiocarracedo/skill-organizer/commit/0d701423760a178f40090e70270a2c8cbaf28095))
* **06-02:** show default model in telemetry status output ([a02daac](https://github.com/sergiocarracedo/skill-organizer/commit/a02daac16c6add66fb54e541817daee9a5e6b2f1))
* **cli:** add check-security cobra command ([137783b](https://github.com/sergiocarracedo/skill-organizer/commit/137783b39bbac1561d991b66bf3b9e6eef711239))
* **cli:** add internal/security package with prompt builder and report types ([2158be1](https://github.com/sergiocarracedo/skill-organizer/commit/2158be15d1e880caebbe13c2889bf4d40b537bbd))
* **cli:** add post-install check-security hook to skill add ([ef71b34](https://github.com/sergiocarracedo/skill-organizer/commit/ef71b34aabba59dcd0be83871bf7fbaf59acbe80))
* **cli:** add re-enable risk gate ([395dd3a](https://github.com/sergiocarracedo/skill-organizer/commit/395dd3ac8dab65f4ec892ec7db3c37f0792d80b8))
* **cli:** add risk-score fields to ManagedMetadata schema ([e92f857](https://github.com/sergiocarracedo/skill-organizer/commit/e92f857425fa58cb55a453dafc9847892e77086a))
* **cmd:** extend telemetry status with recorder type, account id, and insert key ([ce24622](https://github.com/sergiocarracedo/skill-organizer/commit/ce2462227f6d02a3aa2d251b8ab2c14a7b2f2aa1))
* **cmd:** wire SKILL_ORGANIZER_NEWRELIC_* env vars and RecorderVersion ([226aa00](https://github.com/sergiocarracedo/skill-organizer/commit/226aa0036cd83217433ac6ef9e5ee6819754f5c8))
* **p6-03:** add ComputeSkillHash helper and store hash during check-security ([812e875](https://github.com/sergiocarracedo/skill-organizer/commit/812e87558dd9351c289d47e501aa7f3876b29dec))
* **p6-03:** add risk fields to SkillStatus and populate from metadata ([8ae8382](https://github.com/sergiocarracedo/skill-organizer/commit/8ae8382201bef02be047329eb4bd1dfac6116386))
* **p6-03:** add RiskSourceHash field to ManagedMetadata with YAML round-trip ([142fabd](https://github.com/sergiocarracedo/skill-organizer/commit/142fabdc59d21a0e755d9b60fe5c20d596706b4d))
* **p6-03:** render risk tags in status tree with color-coded scores and stale detection ([da6a48b](https://github.com/sergiocarracedo/skill-organizer/commit/da6a48b020a8c8c5b3d07331e758eab2b5fb158b))
* **p6-06-01:** add DefaultModel/KnownModels to AgentSelectionConfig + round-trip tests ([9d508b9](https://github.com/sergiocarracedo/skill-organizer/commit/9d508b95ce45fe6f3ed2c548622edb58027e6a00))
* **p6-06-01:** add ListModels/ModelArgs/VersionArgs to Tool struct + QueryToolModels helper ([1613c64](https://github.com/sergiocarracedo/skill-organizer/commit/1613c64d81d2a04f83a8c70ca8727572dca54d73))
* **p6-06-01:** create dangerous fixture skills for security evaluator tests ([1967b01](https://github.com/sergiocarracedo/skill-organizer/commit/1967b01e92dcb4b33f77d88b9eb862a1aae8e1d9))
* **p6-06-01:** define per-tool ListModels/ModelArgs/VersionArgs for OpenCode ([597312e](https://github.com/sergiocarracedo/skill-organizer/commit/597312e437676e4b852e9b6ef2ad818f7de8dedc))
* Phase 6 AI model visibility and security tooling ([b92c55a](https://github.com/sergiocarracedo/skill-organizer/commit/b92c55a15e33f1cd8db7afb2a55ee4be1dbbbbe2))
* **telemetry:** add NewRelicRecorder struct, Record method, and WarningFunc ([b910f31](https://github.com/sergiocarracedo/skill-organizer/commit/b910f31a52b2bb80d664ce585ba696c4e8516086))
* **telemetry:** extend RecorderConfig and SetDefaultFactory for NewRelicRecorder ([221fcee](https://github.com/sergiocarracedo/skill-organizer/commit/221fcee8aafc809541dff2bb1839f7fbb6d0d1cf))

## [1.0.3](https://github.com/sergiocarracedo/skill-organizer/compare/v1.0.2...v1.0.3) (2026-05-11)


### Bug Fixes

* refresh service unit before start ([7a5c154](https://github.com/sergiocarracedo/skill-organizer/commit/7a5c1546f43d19617b99d1d5a646297a5145af16))

## [1.0.2](https://github.com/sergiocarracedo/skill-organizer/compare/v1.0.1...v1.0.2) (2026-05-11)


### Bug Fixes

* sync web version badge and command docs ([7f55e08](https://github.com/sergiocarracedo/skill-organizer/commit/7f55e08e8f171c6e72da9548ed2357578d5574f7))

## [1.0.1](https://github.com/sergiocarracedo/skill-organizer/compare/v1.0.0...v1.0.1) (2026-05-11)


### Bug Fixes

* avoid pages deploy during tagged releases ([b985669](https://github.com/sergiocarracedo/skill-organizer/commit/b98566954c2173e096b9f4eda10f9a6269ca4124))
* block prerelease auto-merges into main ([f6173c2](https://github.com/sergiocarracedo/skill-organizer/commit/f6173c2aca2eb73bb8c8800f456313805ef02f73))

## [1.0.2-beta.1](https://github.com/sergiocarracedo/skill-organizer/compare/v1.0.1-beta.1...v1.0.2-beta.1) (2026-05-10)


### Bug Fixes

* archive cli release docs from package ([ca0c44d](https://github.com/sergiocarracedo/skill-organizer/commit/ca0c44d840a10300e65654749978971d66e53dd1))

## [1.0.1-beta.1](https://github.com/sergiocarracedo/skill-organizer/compare/v1.0.0-beta.1...v1.0.1-beta.1) (2026-05-10)


### Bug Fixes

* package release archives correctly ([41ed953](https://github.com/sergiocarracedo/skill-organizer/commit/41ed953d343fcb168b340cc2f1e08cd2471b62c5))

## [1.0.0-beta.1](https://github.com/sergiocarracedo/skill-organizer/compare/v0.0.5...v1.0.0-beta.1) (2026-05-10)


### Features

* add cli self-update command ([bb29d2e](https://github.com/sergiocarracedo/skill-organizer/commit/bb29d2e29ce104600333df1354955b5aef7e9755))
* add cli self-update command ([5980803](https://github.com/sergiocarracedo/skill-organizer/commit/5980803b314a615dae7b89a83e456f33eae77dc2))
* add managed skills.sh import and update workflow ([9048e84](https://github.com/sergiocarracedo/skill-organizer/commit/9048e843a3e7cd3445cceb61f8c5ecc05ff20f5b))
* add workflow ([12473ef](https://github.com/sergiocarracedo/skill-organizer/commit/12473eff9211b909212eb0034704621f591b5bf1))
* check updates ([2f4b791](https://github.com/sergiocarracedo/skill-organizer/commit/2f4b79169a5133b4b08144b6e5ae2eb3a98dd51a))
* prepare beta release candidate ([89e0f21](https://github.com/sergiocarracedo/skill-organizer/commit/89e0f218231bb5ff20621cb74af215ee9cc797a7))


### Bug Fixes

* make cli e2e portable in CI ([66bb5a5](https://github.com/sergiocarracedo/skill-organizer/commit/66bb5a523fa173ea5bfe142c640bc2533a0626e1))

## [0.0.5](https://github.com/sergiocarracedo/skill-organizer/compare/v0.0.4...v0.0.5) (2026-04-29)


### Features

* add skill overlap analysis workflow ([f11bf8d](https://github.com/sergiocarracedo/skill-organizer/commit/f11bf8db3e4cf36e8a6c4eba2f5c6282ea828593))
* expand overlap workflows and shell completion ([3d1a3fd](https://github.com/sergiocarracedo/skill-organizer/commit/3d1a3fd90bef3e29b4d820d8449bcabd6dd37e83))

## [0.0.4](https://github.com/sergiocarracedo/skill-organizer/compare/v0.0.3...v0.0.4) (2026-04-28)


### Features

* polish command output and selector UX ([9f656de](https://github.com/sergiocarracedo/skill-organizer/commit/9f656de88b7828f685bb0207c4a4fb8e14adafe7))
* polish command output and selector UX ([009165f](https://github.com/sergiocarracedo/skill-organizer/commit/009165fc111ad333292d9de6a94ebb351a514aa2))
* polish command output and selector UX ([c888384](https://github.com/sergiocarracedo/skill-organizer/commit/c8883842e84347bcee5bfb2eedd9fc73affe1405))

## [0.0.3](https://github.com/sergiocarracedo/skill-organizer/compare/v0.0.2...v0.0.3) (2026-04-27)


### Features

* improve config discovery and move-unmanaged UX ([68afc24](https://github.com/sergiocarracedo/skill-organizer/commit/68afc24bb5b644f9bff0b5c7b9eea537bdb6e8e8))

## [0.0.2](https://github.com/sergiocarracedo/skill-organizer/compare/v0.0.1...v0.0.2) (2026-04-27)


### Bug Fixes

* trigger patch release ([aa63701](https://github.com/sergiocarracedo/skill-organizer/commit/aa6370166f092509c06c2af584730633426f0401))
* trigger release workflow ([6a0ea30](https://github.com/sergiocarracedo/skill-organizer/commit/6a0ea30ef279b2e6a9a9c0d68efafb2631fe75fc))

## [0.0.1] - 2026-04-27

- Initial stable release.
