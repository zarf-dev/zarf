const sbomSelector = document.getElementById('sbom-selector');
const distroInfo = document.getElementById('distro-info');
const modal = document.getElementById('modal');
const modalFader = document.getElementById('modal-fader');
const modalTitle = document.getElementById('modal-title');
const modalContent = document.getElementById('modal-content');
const artifactsTable = document.createElement('table');
const mailtoMaintainerReplace = `&nbsp;|&nbsp;&nbsp;<a href="mailto:$1">$1</a>`;

document.body.appendChild(artifactsTable);

function fileList(files, artifactName) {
	if (files) {
		const list = (files || []).map((file) => file.path || '').filter((test) => test);

		if (list.length > 0) {
			flatList = list.sort().join('<br>');
			return `<a href="#" onClick="showModal('${
				artifactName
			}','${flatList}')">${list.length} files</a>`;
		}
	}

	return '-';
}

function choose(path) {
	if (path !== '-') {
		window.location.href = encodeURIComponent(`sbom-viewer-${path}.html`);
	}
}

function exportCSV(path) {
	if (window.dt) {
		window.dt.export({
			type: 'csv',
			filename: path
		});
	} else {
		showModal('Unable to Export', 'No data in current table');
	}
}

function showModal(title, list) {
	modalTitle.innerText = `Files for ${title}`;
	modalContent.innerHTML = list;
	modalFader.className = 'active';
	modal.className = 'active';
}

function hideModal() {
	modalFader.className = '';
	modal.className = '';
	modalTitle.innerText = '';
	modalContent.innerHTML = '';
}
