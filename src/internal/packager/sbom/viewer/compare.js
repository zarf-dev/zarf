const sbomSelector = document.getElementById("sbom-selector")
const distroInfo = document.getElementById("distro-info")
const modal = document.getElementById("modal")
const modalFader = document.getElementById("modal-fader")
const modalTitle = document.getElementById("modal-title")
const modalContent = document.getElementById("modal-content")
const artifactsTable = document.createElement("table")

document.body.appendChild(artifactsTable);

function initSelector() {
    sbomSelector.add(new Option("-", "-", true, true));

    ZARF_SBOM_LIST.sort().forEach(item => {
        sbomSelector.add(new Option(item, item, false, false));
    });
}

function compare() {
    if (document.getElementById("leftJson").files.length == 0 || document.getElementById("rightJson").files.length == 0) {
        showModal("Unable to Compare", "You must select 2 files from the file browsers")
        return
    }

    let leftJson = document.getElementById("leftJson").files[0];
    let rightJson = document.getElementById("rightJson").files[0];

    let leftReader = new FileReader();
    leftReader.readAsText(leftJson);

    leftReader.onload = function() {
        try {
            let leftData = JSON.parse(leftReader.result);
            const leftMap = {}
            leftData.artifacts.map(artifact => {
                if (!leftMap[artifact.name]) { leftMap[artifact.name] = {} }
                leftMap[artifact.name][artifact.version] = artifact
            });

            let rightReader = new FileReader();
            rightReader.readAsText(rightJson);

            rightReader.onload = function() {
                try {
                    let rightData = JSON.parse(rightReader.result);
                    const rightMap = {}
                    rightData.artifacts.map(artifact => {
                        if (!rightMap[artifact.name]) { rightMap[artifact.name] = {} }
                        rightMap[artifact.name][artifact.version] = artifact
                    });

                    let differences = [];
                    rightData.artifacts.map(artifact => {
                        if (!leftMap[artifact.name]) {
                            artifact.zarfDiff = "Added"
                            differences.push(artifact)
                        } else if (!leftMap[artifact.name][artifact.version]) {
                            artifact.zarfDiff = "Changed"
                            oldVersion = Object.keys(leftMap[artifact.name])[0]
                            artifact.version = oldVersion + " -> " + artifact.version
                            differences.push(artifact)
                        }
                    });

                    leftData.artifacts.map(artifact => {
                        if (!rightMap[artifact.name]) {
                            artifact.zarfDiff = "Removed"
                            differences.push(artifact)
                        }
                    });

                    loadDataTable(differences, artifactsTable)
                } catch (e) {
                    showModal("Unable to Compare", "You must select 2 Syft JSON files")
                }
            };
        } catch (e) {
            showModal("Unable to Compare", "You must select 2 Syft JSON files")
        }
    };
}

function loadDataTable(artifacts, dataTable) {
    const transformedData = artifacts.map(artifact => {
        return [
            diff(artifact.zarfDiff),
            artifact.type,
            artifact.name,
            artifact.version,
            fileList(artifact.metadata),
            (artifact.metadata && artifact.metadata.description) || "-",
            ((artifact.metadata && artifact.metadata.maintainer) || "-").replace(/\u003c(.*)\u003e/, `&nbsp;|&nbsp;&nbsp;<a href="mailto:$1">$1</a>`),
            (artifact.metadata && artifact.metadata.installedSize) || "-",
        ];
    });

    const data = {
        "headings": [
            "Difference",
            "Type",
            "Name",
            "Version",
            "Files",
            "Notes",
            "Maintainer",
            "Size",
        ],
        "data": transformedData,
    };

    if (window.dt) {
        window.dt.destroy();
    }

    window.dt = new simpleDatatables.DataTable(dataTable, {
        data,
        perPage: 20,
    });
}

function fileList(metadata) {
    if (metadata) {
        const list = (metadata.files || [])
            .map(file => {
                return file.path || "";
            })
            .filter(test => test);

        if (list.length > 0) {
            flatList = list.sort().join("<br>");
            return `<a href="#" onClick="showModal('Files for ${metadata.package || metadata.name}','${flatList}')">${list.length} files</a>`;
        }
    }

    return "-";
}

function diff(diffTag) {
    return `<span class="${diffTag.toLowerCase()}">${diffTag}</span>`;
}

function exportCSV() {
    if (window.dt) {
        window.dt.export({
            type: "csv",
            filename: "zarf-sbom-comparison"
        });
    } else {
        showModal("Unable to Export", "No data in current table");
    }
}

function choose(path) {
    if (path !== "-") {
        window.location.href = encodeURIComponent(`sbom-viewer-${path}.html`);
    }
}

function showModal(title, list) {
    modalTitle.innerText = title;
    modalContent.innerHTML = list;
    modalFader.className = "active";
    modal.className = "active";
}

function hideModal() {
    modalFader.className = "";
    modal.className = "";
    modalTitle.innerText = "";
    modalContent.innerHTML = "";
}

initSelector();
