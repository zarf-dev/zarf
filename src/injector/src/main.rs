// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

use std::env;
use std::fs;
use std::fs::File;
use std::io;
use std::io::Read;
use std::io::Write;
use std::path::PathBuf;

use axum::{
    body::Body,
    extract::Path,
    http::StatusCode,
    response::{IntoResponse, Response},
    routing::get,
    Router,
};
use flate2::read::GzDecoder;
use glob::glob;
use hex::ToHex;
use regex_lite::Regex;
use serde_json::Value;
use sha2::{Digest, Sha256};
use tar::Archive;
<<<<<<< HEAD

=======
use tokio_util::io::ReaderStream;
>>>>>>> c0b58b2b (bug: fix rust injector)
const OCI_MIME_TYPE: &str = "application/vnd.oci.image.manifest.v1+json";

// Reads the binary contents of a file
fn get_file(path: &PathBuf) -> io::Result<Vec<u8>> {
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

// Merges all given files into one buffer
fn collect_binary_data(paths: &Vec<PathBuf>) -> io::Result<Vec<u8>> {
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

/// Unpacks the zarf-payload-* configmaps back into a tarball, then unpacks into the CWD
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
        .unpack("/zarf-seed")
        .expect("Unable to unarchive the resulting tarball");
}

/// Starts a static docker compliant registry server that only serves the single image from the CWD
///
/// (which is a OCI image layout):
///
/// index.json - the image index
/// blobs/sha256/<sha256sum> - the image layers
/// oci-layout - the OCI image layout
fn start_seed_registry() -> Router {
    // The name and reference parameter identify the image
    // The reference may include a tag or digest.
    Router::new()
        .route("/v2/*path", get(handler))
        .route(
            "/v2/",
            get(|| async {
                Response::builder()
                    .status(StatusCode::OK)
                    .header("Content-Type", "application/json; charset=utf-8")
                    .header("Docker-Distribution-Api-Version", "registry/2.0")
                    .header("X-Content-Type-Options", "nosniff")
                    .body(Body::empty())
                    .unwrap()
            }),
        )
        .route(
            "/v2",
            get(|| async {
                Response::builder()
                    .status(StatusCode::OK)
                    .header("Content-Type", "application/json; charset=utf-8")
                    .header("Docker-Distribution-Api-Version", "registry/2.0")
                    .header("X-Content-Type-Options", "nosniff")
                    .body(Body::empty())
                    .unwrap()
            }),
        )
}

async fn handler(Path(path): Path<String>) -> Response {
    println!("request: {}", path);
    let path = &path;
    let manifest = Regex::new("(.+)/manifests/(.+)").unwrap();
    let blob = Regex::new(".+/([^/]+)").unwrap();

<<<<<<< HEAD
    if url_seg_len >= 4 && url_segments[1] == "v2" {
        let tag_index = url_seg_len - 1;
        let object_index = url_seg_len - 2;

        let object_type = url_segments[object_index];

        if object_type == "manifests" {
            let tag_or_digest = url_segments[tag_index].to_owned();

            let namespaced_name = url_segments[2..object_index].join("/");

            // this route handles (GET) (/v2/**/manifests/<tag>)
            if request.method() == "GET" {
                return handle_get_manifest(&root, &namespaced_name, &tag_or_digest);
            // this route handles (HEAD) (/v2/**/manifests/<tag>)
            } else if request.method() == "HEAD" {
                // a normal HEAD response has an empty body, but due to rouille not allowing for an override
                // on Content-Length, we respond the same as a GET
                return accept!(
                    request,
                    OCI_MIME_TYPE => {
                        handle_get_manifest(&root, &namespaced_name, &tag_or_digest)
                    },
                    "*/*" => Response::empty_406()
                );
            }
        // this route handles (GET) (/v2/**/blobs/<digest>)
        } else if object_type == "blobs" && request.method() == "GET" {
            let digest = url_segments[tag_index].to_owned();
            return handle_get_digest(&root, &digest);
        }
=======
    if manifest.is_match(path) {
        let caps = manifest.captures(path).unwrap();
        let name = caps.get(1).unwrap().as_str().to_string();
        let reference = caps.get(2).unwrap().as_str().to_string();
        handle_get_manifest(name, reference).await
    } else if blob.is_match(&path) {
        let caps = blob.captures(path).unwrap();
        let tag = caps.get(1).unwrap().as_str().to_string();
        handle_get_digest(tag).await
    } else {
        Response::builder()
            .status(StatusCode::NOT_FOUND)
            .body(format!("Not Found"))
            .unwrap()
            .into_response()
>>>>>>> c0b58b2b (bug: fix rust injector)
    }
}

/// Handles the GET request for the manifest (only returns a OCI manifest regardless of Accept header)
async fn handle_get_manifest(name: String, reference: String) -> Response {
    let index = fs::read_to_string(PathBuf::from("/zarf-seed").join("index.json"))
        .expect("index.json is read");
    let json: Value = serde_json::from_str(&index).expect("unable to parse index.json");

    let mut sha_manifest: String = "".to_owned();

    if reference.starts_with("sha256:") {
        sha_manifest = reference.strip_prefix("sha256:").unwrap().to_owned();
    } else {
        for manifest in json["manifests"].as_array().unwrap() {
            let image_base_name = manifest["annotations"]["org.opencontainers.image.base.name"]
                .as_str()
                .unwrap();
            let requested_reference = name.to_owned() + ":" + &reference;
            if requested_reference == image_base_name {
                sha_manifest = manifest["digest"]
                    .as_str()
                    .unwrap()
                    .strip_prefix("sha256:")
                    .unwrap()
                    .to_owned();
            }
        }
    }
<<<<<<< HEAD

    if sha_manifest != "" {
        let file = File::open(&root.join("blobs").join("sha256").join(&sha_manifest)).unwrap();
        Response::from_file(OCI_MIME_TYPE, file)
            .with_additional_header(
                "Docker-Content-Digest",
                format!("sha256:{}", sha_manifest.to_owned()),
            )
            .with_additional_header("Etag", format!("sha256:{}", sha_manifest))
            .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
=======
    if sha_manifest.is_empty() {
        Response::builder()
            .status(StatusCode::NOT_FOUND)
            .body(format!("Not Found"))
            .unwrap()
            .into_response()
>>>>>>> c0b58b2b (bug: fix rust injector)
    } else {
        let file_path = PathBuf::from("/zarf-seed")
            .to_owned()
            .join("blobs")
            .join("sha256")
            .join(&sha_manifest);
        match tokio::fs::File::open(&file_path).await {
            Ok(file) => {
                let metadata = match file.metadata().await {
                    Ok(meta) => meta,
                    Err(_) => {
                        return Response::builder()
                            .status(StatusCode::INTERNAL_SERVER_ERROR)
                            .body("Failed to get file metadata".into())
                            .unwrap()
                    }
                };
                let stream = ReaderStream::new(file);
                Response::builder()
                    .status(StatusCode::OK)
                    .header("Content-Type", OCI_MIME_TYPE)
                    .header("Content-Length", metadata.len())
                    .header(
                        "Docker-Content-Digest",
                        format!("sha256:{}", sha_manifest.clone()),
                    )
                    .header("Etag", format!("sha256:{}", sha_manifest))
                    .header("Docker-Distribution-Api-Version", "registry/2.0")
                    .body(Body::from_stream(stream))
                    .unwrap()
            }
            Err(err) => Response::builder()
                .status(StatusCode::NOT_FOUND)
                .body(format!("File not found: {}", err))
                .unwrap()
                .into_response(),
        }
    }
}

/// Handles the GET request for a blob
async fn handle_get_digest(tag: String) -> Response {
    let blob_root = PathBuf::from("/zarf-seed").join("blobs").join("sha256");
    let path = blob_root.join(tag.strip_prefix("sha256:").unwrap());

    match tokio::fs::File::open(&path).await {
        Ok(file) => {
            let stream = ReaderStream::new(file);
            Response::builder()
                .status(StatusCode::OK)
                .header("Content-Type", "application/octet-stream")
                .header("Docker-Content-Digest", tag.to_owned())
                .header("Etag", tag.to_owned())
                .header("Docker-Distribution-Api-Version", "registry/2.0")
                .header("Cache-Control", "max-age=31536000")
                .body(Body::from_stream(stream))
                .unwrap()
        }
        Err(err) => Response::builder()
            .status(StatusCode::NOT_FOUND)
            .body(format!("File not found: {}", err))
            .unwrap()
            .into_response(),
    }
}

#[tokio::main(flavor = "current_thread")]
async fn main() {
    let args: Vec<String> = env::args().collect();

    println!("unpacking: {}", args[1]);
    let payload_sha = &args[1];

    unpack(payload_sha);

    let listener = tokio::net::TcpListener::bind("0.0.0.0:5000").await.unwrap();
    println!("listening on {}", listener.local_addr().unwrap());
    axum::serve(listener, start_seed_registry()).await.unwrap();
    println!("Usage: {} <sha256sum>", args[1]);
}
