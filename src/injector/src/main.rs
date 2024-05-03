// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

use std::env;
use std::fs;
use std::fs::File;
use std::io;
use std::io::Read;
use std::io::Write;
use std::path::{Path, PathBuf};

use flate2::read::GzDecoder;
use glob::glob;
use hex::ToHex;
use rouille::{accept, router, Request, Response};
use serde_json::Value;
use sha2::{Digest, Sha256};
use tar::Archive;

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
fn start_seed_registry() {
    let root = PathBuf::from("/zarf-seed");
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

                _ => {
                    handle_request(&root, &request)
                }
            )
        })
    });
}

fn handle_request(root: &Path, request: &Request) -> Response {
    let url = request.url();
    let url_segments: Vec<_> = url.split("/").collect();
    let url_seg_len = url_segments.len();

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
    }

    Response::empty_404()
}

/// Handles the GET request for the manifest (only returns a OCI manifest regardless of Accept header)
fn handle_get_manifest(root: &Path, name: &String, tag: &String) -> Response {
    let index = fs::read_to_string(root.join("index.json")).expect("read index.json");
    let json: Value = serde_json::from_str(&index).expect("unable to parse index.json");
    let mut sha_manifest = "".to_owned();

    if tag.starts_with("sha256:") {
        sha_manifest = tag.strip_prefix("sha256:").unwrap().to_owned();
    } else {
        for manifest in json["manifests"].as_array().unwrap() {
            let image_base_name = manifest["annotations"]["org.opencontainers.image.base.name"]
                .as_str()
                .unwrap();
            let requested_reference = name.to_owned() + ":" + tag;
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

    if sha_manifest != "" {
        let file = File::open(&root.join("blobs").join("sha256").join(&sha_manifest)).unwrap();
        Response::from_file(OCI_MIME_TYPE, file)
            .with_additional_header(
                "Docker-Content-Digest",
                format!("sha256:{}", sha_manifest.to_owned()),
            )
            .with_additional_header("Etag", format!("sha256:{}", sha_manifest))
            .with_additional_header("Docker-Distribution-Api-Version", "registry/2.0")
    } else {
        Response::empty_404()
    }
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

    match args.len() {
        2 => {
            let payload_sha = &args[1];
            unpack(payload_sha);

            start_seed_registry();
        }
        _ => {
            println!("Usage: {} <sha256sum>", args[0]);
        }
    }
}
