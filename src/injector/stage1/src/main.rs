use flate2::read::GzDecoder;
use glob::glob;
use hex::ToHex;
use rouille::{router, Response};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::env;
use std::fs;
use std::fs::File;
use std::io;
use std::io::Read;
use std::io::Write;
use std::path::{Path, PathBuf};
use tar::Archive;

/// Reads the binary contents of a file
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

/// Merges all given files into one buffer
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

/// Unpacks the zarf-payload-* configmaps back into a tarball, then unpacks into /zarf/stage2
///
/// Inspired by https://medium.com/@nlauchande/rust-coding-up-a-simple-concatenate-files-tool-and-first-impressions-a8cbe680e887
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
}

/// Starts a static docker compliant registry server that only serves the single image from /zarf-stage2
///
/// (which is a OCI image layout):
///
/// index.json - the image index
/// blobs/sha256/<sha256sum> - the image layers
/// oci-layout - the OCI image layout
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

/// Handles the GET request for the manifest (only returns a OCI manifest regardless of Accept header)
fn handle_get_manifest(root: &Path) -> Response {
    let index = fs::read_to_string(root.join("index.json")).expect("read index.json");
    let json: Value = serde_json::from_str(&index).expect("unable to parse index.json");
    let sha_manifest = json["manifests"][0]["digest"]
        .as_str()
        .unwrap()
        .strip_prefix("sha256:")
        .unwrap()
        .to_owned();
    let file = File::open(&root.join("blobs").join("sha256").join(&sha_manifest)).unwrap();
    Response::from_file("application/vnd.oci.image.manifest.v1+json", file)
        .with_additional_header(
            "Docker-Content-Digest",
            format!("sha256:{}", sha_manifest.to_owned()),
        )
        .with_additional_header("Etag", format!("sha256:{}", sha_manifest))
        .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
}

/// Handles the GET request for a blob
fn handle_get_digest(root: &Path, digest: &String) -> Response {
    let blob_root = root.join("blobs").join("sha256");
    let path = blob_root.join(digest.strip_prefix("sha256:").unwrap());

    let file = File::open(&path).unwrap();
    Response::from_file("application/octet-stream", file)
        .with_additional_header("Docker-Content-Digest", digest.to_owned())
        .with_additional_header("Etag", digest.to_owned())
        .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
        .with_additional_header("Cache-Control", "max-age=31536000")
}

fn main() {
    let args: Vec<String> = env::args().collect();
    let cmd = &args[1];

    if cmd == "unpack" {
        let sha_sum = &args[2];
        unpack(sha_sum);
    } else if cmd == "serve" {
        let root = Path::new("/zarf-stage2").to_owned();
        start_seed_registry(&root);
    }
}
