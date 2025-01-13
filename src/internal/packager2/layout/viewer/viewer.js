function initSelector() {
	const url = /sbom-viewer-(.*).html*$/gim.exec(window.location.href)[1];

	ZARF_SBOM_LIST.sort().forEach((item) => {
		let selected = url === item ? 'selected' : '';
		sbomSelector.add(new Option(item, item, selected, selected));
	});
}

function initData() {
	const payload = ZARF_SBOM_DATA;

	const transformedData = payload.artifacts.map((artifact) => {
		return [
			artifact.type,
			artifact.name,
			artifact.version,
			fileList(artifact.locations, artifact.name),
			(artifact.metadata && fileList(artifact.metadata.files, artifact.name)) || '-',
			(artifact.metadata && artifact.metadata.description) || '-',
			((artifact.metadata && artifact.metadata.maintainer) || '-').replace(
				/\u003c(.*)\u003e/,
				mailtoMaintainerReplace
			),
			(artifact.metadata && artifact.metadata.installedSize) || '-'
		];
	});

	const data = {
		headings: ['Type', 'Name', 'Version', 'Sources', 'Package Files', 'Notes', 'Maintainer', 'Size'],
		data: transformedData
	};

	if (window.dt) {
		window.dt.destroy();
	}

	distroInfo.innerHTML = payload.distro.prettyName || 'No Base Image Detected';

	window.dt = new simpleDatatables.DataTable(artifactsTable, {
		data,
		perPage: 20
	});
}

function compare() {
	window.location.href = 'compare.html';
}

initSelector();
initData();
