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
use tokio_util::io::ReaderStream;
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
    let init_root =
        std::env::var("ZARF_INJECTOR_INIT_ROOT").unwrap_or_else(|_| String::from("/zarf-init"));
    let seed_root =
        std::env::var("ZARF_INJECTOR_SEED_ROOT").unwrap_or_else(|_| String::from("/zarf-seed"));

    // get the list of file matches to merge
    let glob_path = format!("{}/zarf-payload-*", init_root);
    let file_partials: Result<Vec<_>, _> = glob(&glob_path)
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
        .unpack(seed_root)
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
    }
}

/// Handles the GET request for the manifest (only returns a OCI manifest regardless of Accept header)
async fn handle_get_manifest(name: String, reference: String) -> Response {
    let root = PathBuf::from(
        std::env::var("ZARF_INJECTOR_SEED_ROOT").unwrap_or_else(|_| String::from("/zarf-seed")),
    );

    let index = fs::read_to_string(root.join("index.json")).expect("index.json is read");
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
    if sha_manifest.is_empty() {
        Response::builder()
            .status(StatusCode::NOT_FOUND)
            .body(format!("Not Found"))
            .unwrap()
            .into_response()
    } else {
        let file_path = root.join("blobs").join("sha256").join(&sha_manifest);
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
    let root = PathBuf::from(
        std::env::var("ZARF_INJECTOR_SEED_ROOT").unwrap_or_else(|_| String::from("/zarf-seed")),
    );
    let blob_root = root.join("blobs").join("sha256");
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

#[cfg(test)]
mod test {
    use anyhow::{bail, Context as _, Ok, Result};
    use bollard::{image::CreateImageOptions, Docker};
    use flate2::{write::GzEncoder, Compression};
    use futures_util::{future::ready, TryStreamExt};
    use sha2::{Digest, Sha256};
    use std::{
        fs::File,
        io::{BufRead, BufReader, Cursor, Seek, Write},
        path::{Path, PathBuf},
    };
    use temp_dir::TempDir;

    use crate::{start_seed_registry, unpack};

    // TODO: Make this configurable?
    const TEST_IMAGE: &str = "ghcr.io/zarf-dev/doom-game:0.0.1";
    // Split gzip into 1024 * 768 kb chunks
    const CHUNK_SIZE: usize = 1024 * 768;
    const ZARF_PAYLOAD_PREFIX: &str = "zarf-payload";
    // Based on upstream rust-oci-client regex:
    // https://github.com/oras-project/rust-oci-client/blob/657c1caf9e99ce2184a96aa319fde4f4a8c09439/src/regexp.rs#L3-L5
    const REFERENCE_REGEXP: &str = r"^((?:(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])(?:(?:\.(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]))+)?(?::[0-9]+)?/)?[a-z0-9]+(?:(?:(?:[._]|__|[-]*)[a-z0-9]+)+)?(?:(?:/[a-z0-9]+(?:(?:(?:[._]|__|[-]*)[a-z0-9]+)+)?)+)?)(?::([\w][\w.-]{0,127}))?(?:@([A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}))?$";

    #[tokio::test]
    async fn test_integration() {
        let docker = Docker::connect_with_socket_defaults()
            .expect("should have been able to create a Docker client");
        let tmpdir = TempDir::new().expect("should have created a temporary directory");

        let env = TestEnv::new(docker.clone(), TEST_IMAGE, tmpdir.path().to_owned())
            .await
            .expect("should have setup the test environment");

        let output_root = env.output_dir();
        std::env::set_var("ZARF_INJECTOR_INIT_ROOT", env.input_dir());
        std::env::set_var("ZARF_INJECTOR_SEED_ROOT", &output_root);
        unpack(&env.shasum());

        // Assert the files and directory we expect to exist do exist
        assert!(Path::new(&output_root.join("index.json")).exists());
        assert!(Path::new(&output_root.join("manifest.json")).exists());
        assert!(Path::new(&output_root.join("oci-layout")).exists());
        assert!(Path::new(&output_root.join("repositories")).exists());
        // TODO: Assert all of the blobs referenced in index.json and manifest.json exist under blobs/sha256/...

        localize_test_image(TEST_IMAGE, &output_root)
            .expect("should have localized the test image's index.json");

        // Use :0 to let the operating system decide the random port to listen on
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
            .await
            .expect("should have been able to bind listener to a random port on localhost");
        let random_port = listener
            .local_addr()
            .expect("should have been able to resolve the address")
            .port();

        // Start registry in the background
        tokio::spawn(async {
            let app = start_seed_registry();
            axum::serve(listener, app)
                .await
                .expect("should have been able to start serving the registry");
        });

        let test_image = TEST_IMAGE.replace("ghcr.io", &format!("127.0.0.1:{random_port}"));
        let options = Some(CreateImageOptions {
            from_image: test_image.clone(),
            ..Default::default()
        });

        let test_image_pull = docker
            .create_image(options, None, None)
            .try_collect::<Vec<_>>()
            .await;
        assert!(test_image_pull.is_ok());
        docker
            .remove_image(&test_image, None, None)
            .await
            .expect("should have cleaned up the pulled test image");
    }

    // This localizes the test image's index.json such that the registry server
    // will be able to match the test image from it
    fn localize_test_image(image_reference: &str, image_root: &Path) -> Result<()> {
        let reference = normalize_manifest_reference(image_reference)
            .context("should have localized the test image reference")?;

        let mut index_file = File::options()
            .read(true)
            .write(true)
            .open(image_root.join("index.json"))
            .context("should have opened index.json")?;

        let mut index_json: serde_json::Value =
            serde_json::from_reader(index_file.try_clone().unwrap())
                .context("should have read index.json")?;

        // Overwrite or add an annotation for "org.opencontainers.image.base.name"
        // that is normalized to be without registry address so that it can be
        // pulled locally
        index_json
            .get_mut("manifests")
            .and_then(|manifests| manifests.get_mut(0))
            .and_then(|array| array.get_mut("annotations"))
            .and_then(|annotations| annotations.as_object_mut())
            .and_then(|annotations| {
                annotations.insert(
                    "org.opencontainers.image.base.name".into(),
                    reference.into(),
                )
            });

        // Rewind index.json so serde overwrites from beginning of the file instead of appending to the end
        index_file.rewind().unwrap();
        serde_json::to_writer(index_file.try_clone().unwrap(), &index_json)
            .context("should have overwrote index.json")?;
        Ok(())
    }

    // "Normalizes" the image reference by removing the registry component from it,
    // so that it can be used for referring to local images.
    fn normalize_manifest_reference(identifier: &str) -> Result<String> {
        let re = regex::Regex::new(REFERENCE_REGEXP)?;
        let caps = re
            .captures(identifier)
            .context("should have matched captures for extracting reference components")?;
        let repository = &caps[1];
        let tag = caps.get(2).map(|m| m.as_str().to_owned());
        let digest = caps.get(3).map(|m| m.as_str().to_owned());
        let reference = match (tag, digest) {
            (None, None) => "latest".into(),
            (None, Some(dgst)) => dgst,
            (Some(tg), None) => tg,
            // This should never happen, but for the sake of satisfying the borrow checker we need it here.
            _ => {
                bail!("both tag and digest were matched by the regex, that should not be possible")
            }
        };
        let name = extract_name(repository);
        Ok(format!("{name}:{reference}"))
    }

    // Based on rust-oci-client's split_domain:
    // https://github.com/oras-project/rust-oci-client/blob/657c1caf9e99ce2184a96aa319fde4f4a8c09439/src/reference.rs#L297-L330
    fn extract_name(name: &str) -> String {
        let mut domain: String;
        let mut remainder: String;

        match name.split_once('/') {
            None => {
                domain = "docker.io".into();
                remainder = name.into();
            }
            Some((left, right)) => {
                if !(left.contains('.') || left.contains(':')) && left != "localhost" {
                    domain = "docker.io".into();
                    remainder = name.into();
                } else {
                    domain = left.into();
                    remainder = right.into();
                }
            }
        }
        if domain == "index.docker.io" {
            domain = "docker.io".into();
        }
        if domain == "docker.io" && !remainder.contains('/') {
            remainder = format!("{}/{}", "library", remainder);
        }

        remainder
    }

    struct TestEnv {
        digest: String,
        input_dir: PathBuf,
        output_dir: PathBuf,
    }

    impl TestEnv {
        async fn new(client: Docker, image: &str, root: PathBuf) -> Result<Self> {
            // Ensure we have test directories set up
            let input_dir = root.join("zarf-init");
            let output_dir = root.join("zarf-seed");
            std::fs::create_dir(&input_dir).context("should have created test input directory")?;
            std::fs::create_dir(&output_dir)
                .context("should have created test output directory")?;

            // Download test image
            Self::ensure_image_exists_locally(&client, image)
                .await
                .context("should have pulled down the test image")?;

            // Export test image from docker into a stream to iterate over
            let image_stream = client.export_image(image).map_err(anyhow::Error::msg);

            // Create an in-memory seekable buffer reading in the image and
            // for iteration later when creating the zarf-payload-* chunks
            let buffer = Cursor::new(Vec::new());

            // Encode test image as gzip into the buffer
            let mut gz = GzEncoder::new(buffer, Compression::default());
            image_stream
                .try_for_each(|data| {
                    // We map the error to make sure we're propagating the
                    // same type of error across the board
                    let res = gz.write_all(&data).map_err(anyhow::Error::msg);
                    // Ready needs to be called for the stream to do its thing
                    ready(res)
                })
                .await?;

            let mut buffer = gz
                .finish()
                .context("should have finished reading from stream")?;

            // Rewind to the beginning of the now gzip encoded contents image,
            // so that it can be iterated over to create zarf-payload-* chunks
            buffer
                .rewind()
                .context("should have rewound buffer for reading")?;
            let mut reader = BufReader::with_capacity(CHUNK_SIZE, buffer);

            let mut hasher = Sha256::new();
            let mut chunk_id = 0;
            while let std::result::Result::Ok(chunk) = reader.fill_buf() {
                let read_bytes = chunk.len();
                if read_bytes == 0 {
                    break;
                }

                hasher.update(chunk);

                // Write chunks to disk as zarf-payload-00X in temp dir
                let mut chunk_file = File::create(
                    input_dir.join(format!("{}-{:0>3}", ZARF_PAYLOAD_PREFIX, chunk_id)),
                )
                .context("should have created chunk file")?;
                chunk_file
                    .write_all(chunk)
                    .context("should have written chunk to file")?;
                chunk_file
                    .flush()
                    .context("should have flushed chunk file")?;
                chunk_id += 1;

                reader.consume(read_bytes);
            }
            let hash = hasher.finalize();
            let digest = format!("{hash:x}");

            Ok(Self {
                digest,
                input_dir,
                output_dir,
            })
        }

        fn shasum(&self) -> String {
            self.digest.to_owned()
        }

        fn input_dir(&self) -> PathBuf {
            self.input_dir.to_owned()
        }

        fn output_dir(&self) -> PathBuf {
            self.output_dir.to_owned()
        }

        async fn ensure_image_exists_locally(client: &Docker, image: &str) -> Result<()> {
            // Check if the test image already exists.
            if (client.inspect_image(image).await).is_ok() {
                Ok(())
            } else {
                let options = Some(CreateImageOptions {
                    from_image: image,
                    ..Default::default()
                });
                // Attempt to pull image from the upstream registry
                let _ = client
                    .create_image(options, None, None)
                    .try_collect::<Vec<_>>()
                    .await
                    .map_err(anyhow::Error::msg)
                    .context("should have been able to pull test image")?;
                // Inspect the image to make sure it exists locally and then discard the output
                Ok(client.inspect_image(image).await.map(|_| ())?)
            }
        }
    }
}
