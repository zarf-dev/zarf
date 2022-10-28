use flate2::read::GzDecoder;
use glob::glob;
use hex::ToHex;
use oci_spec::image::{
    Descriptor, DescriptorBuilder, ImageManifestBuilder, MediaType, SCHEMA_VERSION,
};
use rouille::{router, Response};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::env;
use std::fs;
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
        let new_content = get_file(path);
        buffer
            .write(&new_content.unwrap())
            .expect("Could not add the file contents to the merged file buffer");
    }

    Ok(buffer)
}

fn unpack(sha_sum: &String) {
    // get the list of file matches to merge
    let file_partials: Result<Vec<_>, _> = glob("zarf-payload-*")
        .expect("Failed to read glob pattern")
        .collect();

    let mut file_partials = file_partials.unwrap();

    // ensure a default sort-order
    file_partials.sort();

    // get a buffer of the final merged file contents
    let contents = collect_binary_data(&file_partials).unwrap();

    // create a Sha256 object
    let mut hasher = Sha256::new();

    // write input message
    hasher.update(&contents);

    // read hash digest and consume hasher
    let result = hasher.finalize();
    let result_string = result.encode_hex::<String>();
    assert_eq!(*sha_sum, result_string);

    // write the merged file to disk and extract it
    let tar = GzDecoder::new(&contents[..]);
    let mut archive = Archive::new(tar);
    archive
        .unpack("/zarf-stage2")
        .expect("Unable to unarchive the resulting tarball");

    let mut seed_image_tar = Archive::new(File::open("/zarf-stage2/seed-image.tar").unwrap());
    seed_image_tar
        .unpack("/zarf-stage2/seed-image")
        .expect("Unable to unarchive the seed image tarball");
}

fn start_seed_registry(file_root: &Path) {
    let root = PathBuf::from(file_root);
    println!("Starting seed registry at {} on port 5000", root.display());
    rouille::start_server("0.0.0.0:5000", move |request| {
        rouille::log(request, io::stdout(), || {
            router!(request,
                (GET) (/v2/) => {
                    // returns empty json w/ Docker-Distribution-Api-Version header set
                    Response::text("{}")
                    .with_unique_header("Content-Type", "application/json; charset=utf-8")
                    .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
                    .with_additional_header("X-Content-Type-Options", "nosniff")
                },

                (GET) (/v2/registry/manifests/{_tag :String}) => {
                    handle_get_manifest(&root)
                },

                (GET) (/v2/{_namespace :String}/registry/manifests/{_ref :String}) => {
                    handle_get_manifest(&root)
                },

                (HEAD) (/v2/registry/manifests/{_ref :String}) => {
                    // a normal HEAD response has an empty body, but due to rouille not allowing for an override
                    // on Content-Length, we respond the same as a GET
                    handle_get_manifest(&root)
                },

                (HEAD) (/v2/{_namespace :String}/registry/manifests/{_ref :String}) => {
                    // a normal HEAD response has an empty body, but due to rouille not allowing for an override
                    // on Content-Length, we respond the same as a GET
                    handle_get_manifest(&root)
                },

                (GET) (/v2/registry/blobs/{digest :String}) => {
                    handle_get_digest(&root, &digest)
                },

                (GET) (/v2/{_namespace :String}/registry/blobs/{digest :String}) => {
                    handle_get_digest(&root, &digest)
                },

                _ => {
                    Response::empty_404()
                }
            )
        })
    });
}

fn handle_get_manifest(root: &Path) -> Response {
    let sha_manifest = fs::read_to_string(root.join("link")).expect("unable to read pointer file");
    let file = File::open(&root.join(&sha_manifest)).unwrap();
    Response::from_file("application/vnd.docker.distribution.manifest.v2+json", file)
        .with_additional_header("Docker-Content-Digest", sha_manifest.to_owned())
        .with_additional_header("Etag", sha_manifest)
        .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
}

fn handle_get_digest(root: &Path, digest: &String) -> Response {
    let mut path = root.join(digest);

    match path.try_exists() {
        Ok(true) => {
            // means they queried the config json
        }
        _ => {
            // means they queried a layer
            path = root.join(digest.strip_prefix("sha256:").unwrap());
            path.set_extension("tar.gz");
        }
    }
    let file = File::open(&path).unwrap();
    Response::from_file("application/octet-stream", file)
        .with_additional_header("Docker-Content-Digest", digest.to_owned())
        .with_additional_header("Etag", digest.to_owned())
        .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
        .with_additional_header("Cache-Control", "max-age=31536000")
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct CraneManifest {
    config: String,
    repo_tags: Vec<String>,
    layers: Vec<String>,
}

fn get_file_size(path: &PathBuf) -> i64 {
    let metadata = std::fs::metadata(path).unwrap();
    metadata.len() as i64
}

fn create_v2_manifest(root: &Path) {
    let data = fs::read_to_string(root.join("manifest.json")).expect("unable to read pointer file");

    let crane_manifest: Vec<CraneManifest> =
        serde_json::from_str(&data).expect("manifest.json was not of struct CraneManifest");

    let config_digest = root.join(crane_manifest[0].config.clone());

    let config = DescriptorBuilder::default()
        .media_type(MediaType::Other(
            "application/vnd.docker.container.image.v1+json".to_string(),
        ))
        .size(get_file_size(&config_digest))
        .digest(crane_manifest[0].config.clone())
        .build()
        .expect("build config descriptor");

    let layers: Vec<Descriptor> = crane_manifest[0]
        .layers
        .iter()
        .map(|layer| {
            let digest = root.join(layer);
            let full_digest = format!(
                "sha256:{}",
                layer.to_string().strip_suffix(".tar.gz").unwrap()
            );

            const ROOTF_DIFF_TAR_GZIP: &str = "application/vnd.docker.image.rootfs.diff.tar.gzip";

            DescriptorBuilder::default()
                .media_type(MediaType::Other(ROOTF_DIFF_TAR_GZIP.to_string()))
                .size(get_file_size(&digest))
                .digest(full_digest)
                .build()
                .expect("build layer")
        })
        .collect();

    let manifest = ImageManifestBuilder::default()
        .schema_version(SCHEMA_VERSION)
        .media_type(MediaType::Other(
            "application/vnd.docker.distribution.manifest.v2+json".to_string(),
        ))
        .config(config)
        .layers(layers)
        .build()
        .expect("build image manifest");

    println!("{}", manifest.to_string_pretty().unwrap());

    manifest
        .to_file_pretty(root.join("manifestv2.json"))
        .unwrap();

    let mut file = File::open(root.join("manifestv2.json")).unwrap();
    let mut hasher = Sha256::new();
    std::io::copy(&mut file, &mut hasher).unwrap();
    let result = hasher.finalize();
    let sha_digest = format!("sha256:{:x}", result);

    std::fs::rename(root.join("manifestv2.json"), root.join(sha_digest.clone())).unwrap();
    let mut file = File::create(root.join("link")).unwrap();
    file.write_all(sha_digest.as_bytes()).unwrap();
}

fn main() {
    let args: Vec<String> = env::args().collect();
    let cmd = &args[1];
    let sha_sum = &args[2];

    if cmd == "unpack" {
        unpack(sha_sum);
    } else if cmd == "serve" {
        let root = Path::new("/zarf-stage2/seed-image").to_owned();
        create_v2_manifest(&root);
        start_seed_registry(&root);
    }
}
