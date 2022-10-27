use flate2::read::GzDecoder;
use glob::glob;
use hex::ToHex;
use oci_spec::image::{
    Descriptor, DescriptorBuilder, ImageManifestBuilder, MediaType, SCHEMA_VERSION,
};
use rouille::{router, Response, ResponseBody};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::env;
use std::fs::File;
use std::io;
use std::io::Read;
use std::io::Write;
use std::path::{Path, PathBuf};
use tar::Archive;

// Inspired by https://medium.com/@nlauchande/rust-coding-up-a-simple-concatenate-files-tool-and-first-impressions-a8cbe680e887

// read the binary contents of a file
fn get_file(path: &PathBuf) -> std::io::Result<Vec<u8>> {
    // open the file
    let mut f = File::open(path)?;
    // create an empty buffer
    let mut buffer = Vec::new();

    // read the whole file
    match f.read_to_end(&mut buffer) {
        Ok(_) => Ok(buffer),
        Err(e) => Err(e),
    }
}

// merge all given files into one buffer
fn collect_binary_data(paths: &Vec<PathBuf>) -> std::io::Result<Vec<u8>> {
    // create an empty buffer
    let mut buffer = Vec::new();

    // add contents of all files in paths to buffer
    for path in paths {
        println!("Processing {}", path.display());
        let new_content = get_file(&path);
        buffer
            .write(&new_content.unwrap())
            .expect("Could not add the file contents to the merged file buffer");
    }

    Ok(buffer)
}

fn main() {
    let args: Vec<String> = env::args().collect();

    // get the list of file matches to merge
    let file_partials: Result<Vec<_>, _> = glob("zarf-payload-*")
        .expect("Failed to read glob pattern")
        .collect();

    let mut file_partials = file_partials.unwrap();

    // ensure a default sort-order
    file_partials.sort();

    // get a buffer of the final merged file contents
    let contents = collect_binary_data(&file_partials).unwrap();

    // verify sha256sum if it exists
    if args.len() > 1 {
        let sha_sum = &args[1];

        // create a Sha256 object
        let mut hasher = Sha256::new();

        // write input message
        hasher.update(&contents);

        // read hash digest and consume hasher
        let result = hasher.finalize();
        let result_string = result.encode_hex::<String>();
        assert_eq!(*sha_sum, result_string);
    }

    // write the merged file to disk and extract it
    let tar = GzDecoder::new(&contents[..]);
    let mut archive = Archive::new(tar);
    archive
        .unpack("/zarf-stage2")
        .expect("Unable to unarchive the resulting tarball");

    start_seed_registry(Path::new("/zarf-stage2"));
}

fn start_seed_registry(root: &Path) {
    let root = root.to_path_buf();
    rouille::start_server("0.0.0.0:5001", move |request| {
        rouille::log(request, io::stdout(), || {
            router!(request,
                (GET) (/v2) => {
                    // mirror from docker api, redirect to /v2/
                    Response {
                        status_code: 301,
                        data: ResponseBody::from_string("<a href=\"/v2/\">Moved Permanently</a>.\n"),
                        headers: vec![("Location".into(), "/v2/".into())],
                        upgrade: None,
                    }.with_unique_header("Content-Type", "text/html; charset=utf-8")
                },

                (GET) (/v2/) => {
                    // mirror from docker api, returns empty json w/ Docker-Distribution-Api-Version header set
                    Response::text("{}")
                    .with_unique_header("Content-Type", "application/json; charset=utf-8")
                    .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
                    .with_additional_header("X-Content-Type-Options", "nosniff")
                },

                (GET) (/v2/registry/manifests/{_tag :String}) => {
                    let mut file = File::open(root.join("manifest.json")).unwrap();
                    let mut data = String::new();
                    file.read_to_string(&mut data).unwrap();

                    #[derive(Serialize, Deserialize)]
                    #[serde(rename_all = "PascalCase")]
                    struct CraneManifest {
                        config: String,
                        repo_tags: Vec<String>,
                        layers: Vec<String>,
                    }

                    let crane_manifest: Vec<CraneManifest> = serde_json::from_str(&data).expect("manifest.json was not of struct CraneManifest");

                    fn get_file_size(path: &PathBuf) -> i64 {
                        let metadata = std::fs::metadata(path).unwrap();
                        metadata.len() as i64
                    }

                    let config_digest = root.join(crane_manifest[0].config.clone());

                    let config = DescriptorBuilder::default()
                        .media_type(MediaType::ImageConfig)
                        .size(get_file_size(&config_digest))
                        .digest(crane_manifest[0].config.clone())
                        .build()
                        .expect("build config descriptor");

                    let layers: Vec<Descriptor> = crane_manifest[0].layers.iter().map(|layer| {
                        let digest = root.join(layer);
                        let full_digest = format!("sha256:{}", layer.to_string().strip_suffix(".tar.gz").unwrap());

                        const ROOTF_DIFF_TAR_GZIP: &str = "application/vnd.docker.image.rootfs.diff.tar.gzip";

                        DescriptorBuilder::default()
                            .media_type(MediaType::Other(ROOTF_DIFF_TAR_GZIP.to_string()))
                            .size(get_file_size(&digest))
                            .digest(full_digest)
                            .build()
                            .expect("build layer")
                    }).collect();

                    let manifest = ImageManifestBuilder::default()
                        .schema_version(SCHEMA_VERSION)
                        .media_type(MediaType::Other("application/vnd.docker.distribution.manifest.v2+json".to_string()))
                        .config(config)
                        .layers(layers)
                        .build()
                        .expect("build image manifest");

                    let response = Response::json(&manifest)
                        .with_unique_header("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
                        .with_additional_header("Docker-Content-Digest", manifest.config().digest().to_string())
                        .with_additional_header("Etag", manifest.config().digest().to_string())
                        .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0");
                    response
                },

                (GET) (/v2/registry/blobs/{digest :String}) => {
                    let mut path = root.join(digest.clone());

                    match path.try_exists() {
                        Ok(true) => {
                            // means they queried the config json
                        },
                        _ => {
                            // means they queried a layer
                            path = root.join(digest.strip_prefix("sha256:").unwrap());
                            path.set_extension("tar.gz");
                        }
                    }
                    let file = File::open(&path).unwrap();
                        Response::from_file("application/octet-stream", file)
                        .with_additional_header("Docker-Content-Digest", digest.clone())
                        .with_additional_header("Etag", digest)
                        .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
                        .with_additional_header("Cache-Control", "max-age=31536000")
                },

                _ => {
                    Response::empty_404()
                }
            )
        })
    });
}
