If the main.rs is changed like this
-    let listener = tokio::net::TcpListener::bind("0.0.0.0:5000").await.unwrap();
+    let listener = tokio::net::TcpListener::bind("[::]:5000").await.unwrap();

We get the error
