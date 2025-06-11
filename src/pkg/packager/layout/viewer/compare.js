const leftJsonPicker = document.getElementById('leftJson');
const rightJsonPicker = document.getElementById('rightJson');

function initSelector() {
	sbomSelector.add(new Option('-', '-', true, true));

	ZARF_SBOM_LIST.sort().forEach((item) => {
		sbomSelector.add(new Option(item, item, false, false));
	});
}

function compare() {
	if (
		document.getElementById('leftJson').files.length == 0 ||
		document.getElementById('rightJson').files.length == 0
	) {
		showModal('Unable to Compare', 'You must select 2 files from the file browsers');
		return;
	}

	let leftJson = document.getElementById('leftJson').files[0];
	let rightJson = document.getElementById('rightJson').files[0];

	let leftReader = new FileReader();
	leftReader.readAsText(leftJson);

	leftReader.onload = function () {
		try {
			let leftData = JSON.parse(leftReader.result);
			const leftMap = {};
			leftData.artifacts.map((artifact) => {
				if (!leftMap[artifact.name]) {
					leftMap[artifact.name] = {};
				}
				leftMap[artifact.name][artifact.version] = artifact;
			});

			let rightReader = new FileReader();
			rightReader.readAsText(rightJson);

			rightReader.onload = function () {
				try {
					let rightData = JSON.parse(rightReader.result);
					const rightMap = {};
					rightData.artifacts.map((artifact) => {
						if (!rightMap[artifact.name]) {
							rightMap[artifact.name] = {};
						}
						rightMap[artifact.name][artifact.version] = artifact;
					});

					let differences = [];
					rightData.artifacts.map((artifact) => {
						if (!leftMap[artifact.name]) {
							artifact.zarfDiff = 'Added';
							differences.push(artifact);
						} else if (!leftMap[artifact.name][artifact.version]) {
							artifact.zarfDiff = 'Changed';
							oldVersion = Object.keys(leftMap[artifact.name])[0];
							artifact.version = oldVersion + ' -> ' + artifact.version;
							differences.push(artifact);
						}
					});

					leftData.artifacts.map((artifact) => {
						if (!rightMap[artifact.name]) {
							artifact.zarfDiff = 'Removed';
							differences.push(artifact);
						}
					});

					loadDataTable(differences, artifactsTable);
				} catch (e) {
					showModal('Unable to Compare', 'You must select 2 Syft JSON files');
				}
			};
		} catch (e) {
			showModal('Unable to Compare', 'You must select 2 Syft JSON files');
		}
	};
}

function loadDataTable(artifacts, dataTable) {
	const transformedData = artifacts.map((artifact) => {
		return [
			diff(artifact.zarfDiff),
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
		headings: ['Difference', 'Type', 'Name', 'Version', 'Sources', 'Package Files', 'Notes', 'Maintainer', 'Size'],
		data: transformedData
	};

	if (window.dt) {
		window.dt.destroy();
	}

	window.dt = new simpleDatatables.DataTable(dataTable, {
		data,
		perPage: 20
	});
}

function diff(diffTag) {
	return `<span class="${diffTag.toLowerCase()}">${diffTag}</span>`;
}

function getCompareName() {
	leftFilename = leftJsonPicker.value.split('/').pop().split('\\').pop();
	rightFilename = rightJsonPicker.value.split('/').pop().split('\\').pop();
    return leftFilename.replace(/\.json$/, '') + '-' + rightFilename.replace(/\.json$/, '');
}

initSelector();
