// To parse this data:
//
//   import { Convert, APITypes } from "./file";
//
//   const aPITypes = Convert.toAPITypes(json);
//
// These functions will throw an error if the JSON doesn't
// match the expected interface, even if the JSON is valid.

export interface APITypes {
    clusterSummary:    ClusterSummary;
    connectStrings:    { [key: string]: ConnectString };
    zarfCommonOptions: ZarfCommonOptions;
    zarfCreateOptions: ZarfCreateOptions;
    zarfDeployOptions: ZarfDeployOptions;
    zarfPackage:       ZarfPackage;
    zarfState:         ZarfState;
}

export interface ClusterSummary {
    distro:    string;
    hasZarf:   boolean;
    reachable: boolean;
}

export interface ConnectString {
    Description: string;
    Url:         string;
}

export interface ZarfCommonOptions {
    Confirm:       boolean;
    SetVariables:  { [key: string]: string };
    TempDirectory: string;
}

export interface ZarfCreateOptions {
    ImageCachePath:  string;
    Insecure:        boolean;
    OutputDirectory: string;
    SkipSBOM:        boolean;
}

export interface ZarfDeployOptions {
    ApplianceMode: boolean;
    Components:    string;
    NodePort:      string;
    PackagePath:   string;
    Secret:        string;
    SGetKeyPath:   string;
    StorageClass:  string;
}

export interface ZarfPackage {
    /**
     * Zarf-generated package build data
     */
    build?: ZarfBuildData;
    /**
     * List of components to deploy in this package
     */
    components: ZarfComponent[];
    /**
     * Constant template values applied on deploy for K8s resources
     */
    constants?: ZarfPackageConstant[];
    /**
     * The kind of Zarf package
     */
    kind: Kind;
    /**
     * Package metadata
     */
    metadata?: ZarfMetadata;
    /**
     * Special image only used for ZarfInitConfig packages when used with the Zarf Injector
     */
    seed?: string;
    /**
     * Variable template values applied on deploy for K8s resources
     */
    variables?: ZarfPackageVariable[];
}

/**
 * Zarf-generated package build data
 */
export interface ZarfBuildData {
    architecture: string;
    terminal:     string;
    timestamp:    string;
    user:         string;
    version:      string;
}

export interface ZarfComponent {
    /**
     * Helm charts to install during package deploy
     */
    charts?: ZarfChart[];
    /**
     * Specify a path to a public key to validate signed online resources
     */
    cosignKeyPath?: string;
    /**
     * Datasets to inject into a pod in the target cluster
     */
    dataInjections?: ZarfDataInjection[];
    /**
     * Determines the default Y/N state for installing this component on package deploy
     */
    default?: boolean;
    /**
     * Message to include during package deploy describing the purpose of this component
     */
    description?: string;
    /**
     * Files to place on disk during package deployment
     */
    files?: ZarfFile[];
    /**
     * Create a user selector field based on all components in the same group
     */
    group?: string;
    /**
     * List of OCI images to include in the package
     */
    images?: string[];
    /**
     * Import a component from another Zarf package
     */
    import?:    ZarfComponentImport;
    manifests?: ZarfManifest[];
    /**
     * The name of the component
     */
    name: string;
    /**
     * Filter when this component is included in package creation or deployment
     */
    only?: ZarfComponentOnlyTarget;
    /**
     * List of git repos to include in the package
     */
    repos?: string[];
    /**
     * Do not prompt user to install this component
     */
    required?: boolean;
    /**
     * Custom commands to run before or after package deployment
     */
    scripts?: ZarfComponentScripts;
}

export interface ZarfChart {
    /**
     * If using a git repo
     */
    gitPath?: string;
    /**
     * The name of the chart to deploy
     */
    name: string;
    /**
     * The namespace to deploy the chart to
     */
    namespace: string;
    /**
     * The name of the release to create
     */
    releaseName?: string;
    /**
     * The URL of the chart repository or git url if the chart is using a git repo instead of
     * helm repo
     */
    url: string;
    /**
     * List of values files to include in the package
     */
    valuesFiles?: string[];
    /**
     * The version of the chart to deploy
     */
    version: string;
}

export interface ZarfDataInjection {
    /**
     * Compress the data before transmitting using gzip.  Note: this requires support for
     * tar/gzip locally and in the target image.
     */
    compress?: boolean;
    /**
     * A path to a local folder or file to inject into the given target pod + container
     */
    source: string;
    /**
     * The target pod + container to inject the data into
     */
    target: ZarfContainerTarget;
}

/**
 * The target pod + container to inject the data into
 */
export interface ZarfContainerTarget {
    /**
     * The container to target for data injection
     */
    container: string;
    /**
     * The namespace to target for data injection
     */
    namespace: string;
    /**
     * The path to copy the data to in the container
     */
    path: string;
    /**
     * The K8s selector to target for data injection
     */
    selector: string;
}

export interface ZarfFile {
    /**
     * Determines if the file should be made executable during package deploy
     */
    executable?: boolean;
    /**
     * SHA256 checksum of the file if the source is a URL
     */
    shasum?: string;
    /**
     * Local file path or remote URL to add to the package
     */
    source: string;
    /**
     * List of symlinks to create during package deploy
     */
    symlinks?: string[];
    /**
     * The absolute or relative path wher the file should be copied to during package deploy
     */
    target: string;
}

/**
 * Import a component from another Zarf package
 */
export interface ZarfComponentImport {
    name?: string;
    path:  string;
}

export interface ZarfManifest {
    /**
     * List of individual K8s YAML files to deploy (in order)
     */
    files?: string[];
    /**
     * List of kustomization paths to include in the package
     */
    kustomizations?: string[];
    /**
     * Allow traversing directory above the current directory if needed for kustomization
     */
    kustomizeAllowAnyDirectory?: boolean;
    /**
     * A name to give this collection of manifests
     */
    name: string;
    /**
     * The namespace to deploy the manifests to
     */
    namespace?: string;
}

/**
 * Filter when this component is included in package creation or deployment
 */
export interface ZarfComponentOnlyTarget {
    /**
     * Only deploy component to specified clusters
     */
    cluster?: ZarfComponentOnlyCluster;
    /**
     * Only deploy component to specified OS
     */
    localOS?: LocalOS;
}

/**
 * Only deploy component to specified clusters
 */
export interface ZarfComponentOnlyCluster {
    /**
     * Only create and deploy to clusters of the given architecture
     */
    architecture?: Architecture;
    /**
     * Future use
     */
    distros?: string[];
}

/**
 * Only create and deploy to clusters of the given architecture
 */
export enum Architecture {
    Amd64 = "amd64",
    Arm64 = "arm64",
}

/**
 * Only deploy component to specified OS
 */
export enum LocalOS {
    Darwin = "darwin",
    Linux = "linux",
    Windows = "windows",
}

/**
 * Custom commands to run before or after package deployment
 */
export interface ZarfComponentScripts {
    /**
     * Scripts to run after the component successfully deploys
     */
    after?: string[];
    /**
     * Scripts to run before the component is deployed
     */
    before?: string[];
    /**
     * Scripts to run before the component is added during package create
     */
    prepare?: string[];
    /**
     * Retry the script if it fails
     */
    retry?: boolean;
    /**
     * Show the output of the script during package deployment
     */
    showOutput?: boolean;
    /**
     * Timeout in seconds for the script
     */
    timeoutSeconds?: number;
}

export interface ZarfPackageConstant {
    /**
     * The name to be used for the constant
     */
    name: string;
    /**
     * The value to set for the constant during deploy
     */
    value: string;
}

/**
 * The kind of Zarf package
 */
export enum Kind {
    ZarfInitConfig = "ZarfInitConfig",
    ZarfPackageConfig = "ZarfPackageConfig",
}

/**
 * Package metadata
 */
export interface ZarfMetadata {
    /**
     * The target cluster architecture of this package
     */
    architecture?: string;
    /**
     * Additional information about this package
     */
    description?: string;
    /**
     * An image URL to embed in this package for future Zarf UI listing
     */
    image?: string;
    /**
     * Name to identify this Zarf package
     */
    name: string;
    /**
     * Disable compression of this package
     */
    uncompressed?: boolean;
    /**
     * Link to package information when online
     */
    url?: string;
    /**
     * Generic string to track the package version by a package author
     */
    version?: string;
}

export interface ZarfPackageVariable {
    /**
     * The default value to use for the variable
     */
    default?: string;
    /**
     * The name to be used for the variable
     */
    name: string;
    /**
     * Whether to prompt the user for input for this variable
     */
    prompt?: boolean;
}

export interface ZarfState {
    agentTLS: GeneratedPKI;
    /**
     * Machine architecture of the k8s node(s)
     */
    architecture: string;
    /**
     * K8s distribution of the cluster Zarf was deployed to
     */
    distro:       string;
    nodePort:     string;
    secret:       string;
    storageClass: string;
    /**
     * Indicates if Zarf was initialized while deploying its own k8s cluster
     */
    zarfAppliance: boolean;
}

export interface GeneratedPKI {
    CA:   string;
    Cert: string;
    Key:  string;
}

// Converts JSON strings to/from your types
// and asserts the results of JSON.parse at runtime
export class Convert {
    public static toAPITypes(json: string): APITypes {
        return cast(JSON.parse(json), r("APITypes"));
    }

    public static aPITypesToJson(value: APITypes): string {
        return JSON.stringify(uncast(value, r("APITypes")), null, 2);
    }
}

function invalidValue(typ: any, val: any, key: any = ''): never {
    if (key) {
        throw Error(`Invalid value for key "${key}". Expected type ${JSON.stringify(typ)} but got ${JSON.stringify(val)}`);
    }
    throw Error(`Invalid value ${JSON.stringify(val)} for type ${JSON.stringify(typ)}`, );
}

function jsonToJSProps(typ: any): any {
    if (typ.jsonToJS === undefined) {
        const map: any = {};
        typ.props.forEach((p: any) => map[p.json] = { key: p.js, typ: p.typ });
        typ.jsonToJS = map;
    }
    return typ.jsonToJS;
}

function jsToJSONProps(typ: any): any {
    if (typ.jsToJSON === undefined) {
        const map: any = {};
        typ.props.forEach((p: any) => map[p.js] = { key: p.json, typ: p.typ });
        typ.jsToJSON = map;
    }
    return typ.jsToJSON;
}

function transform(val: any, typ: any, getProps: any, key: any = ''): any {
    function transformPrimitive(typ: string, val: any): any {
        if (typeof typ === typeof val) return val;
        return invalidValue(typ, val, key);
    }

    function transformUnion(typs: any[], val: any): any {
        // val must validate against one typ in typs
        const l = typs.length;
        for (let i = 0; i < l; i++) {
            const typ = typs[i];
            try {
                return transform(val, typ, getProps);
            } catch (_) {}
        }
        return invalidValue(typs, val);
    }

    function transformEnum(cases: string[], val: any): any {
        if (cases.indexOf(val) !== -1) return val;
        return invalidValue(cases, val);
    }

    function transformArray(typ: any, val: any): any {
        // val must be an array with no invalid elements
        if (!Array.isArray(val)) return invalidValue("array", val);
        return val.map(el => transform(el, typ, getProps));
    }

    function transformDate(val: any): any {
        if (val === null) {
            return null;
        }
        const d = new Date(val);
        if (isNaN(d.valueOf())) {
            return invalidValue("Date", val);
        }
        return d;
    }

    function transformObject(props: { [k: string]: any }, additional: any, val: any): any {
        if (val === null || typeof val !== "object" || Array.isArray(val)) {
            return invalidValue("object", val);
        }
        const result: any = {};
        Object.getOwnPropertyNames(props).forEach(key => {
            const prop = props[key];
            const v = Object.prototype.hasOwnProperty.call(val, key) ? val[key] : undefined;
            result[prop.key] = transform(v, prop.typ, getProps, prop.key);
        });
        Object.getOwnPropertyNames(val).forEach(key => {
            if (!Object.prototype.hasOwnProperty.call(props, key)) {
                result[key] = transform(val[key], additional, getProps, key);
            }
        });
        return result;
    }

    if (typ === "any") return val;
    if (typ === null) {
        if (val === null) return val;
        return invalidValue(typ, val);
    }
    if (typ === false) return invalidValue(typ, val);
    while (typeof typ === "object" && typ.ref !== undefined) {
        typ = typeMap[typ.ref];
    }
    if (Array.isArray(typ)) return transformEnum(typ, val);
    if (typeof typ === "object") {
        return typ.hasOwnProperty("unionMembers") ? transformUnion(typ.unionMembers, val)
            : typ.hasOwnProperty("arrayItems")    ? transformArray(typ.arrayItems, val)
            : typ.hasOwnProperty("props")         ? transformObject(getProps(typ), typ.additional, val)
            : invalidValue(typ, val);
    }
    // Numbers can be parsed by Date but shouldn't be.
    if (typ === Date && typeof val !== "number") return transformDate(val);
    return transformPrimitive(typ, val);
}

function cast<T>(val: any, typ: any): T {
    return transform(val, typ, jsonToJSProps);
}

function uncast<T>(val: T, typ: any): any {
    return transform(val, typ, jsToJSONProps);
}

function a(typ: any) {
    return { arrayItems: typ };
}

function u(...typs: any[]) {
    return { unionMembers: typs };
}

function o(props: any[], additional: any) {
    return { props, additional };
}

function m(additional: any) {
    return { props: [], additional };
}

function r(name: string) {
    return { ref: name };
}

const typeMap: any = {
    "APITypes": o([
        { json: "clusterSummary", js: "clusterSummary", typ: r("ClusterSummary") },
        { json: "connectStrings", js: "connectStrings", typ: m(r("ConnectString")) },
        { json: "zarfCommonOptions", js: "zarfCommonOptions", typ: r("ZarfCommonOptions") },
        { json: "zarfCreateOptions", js: "zarfCreateOptions", typ: r("ZarfCreateOptions") },
        { json: "zarfDeployOptions", js: "zarfDeployOptions", typ: r("ZarfDeployOptions") },
        { json: "zarfPackage", js: "zarfPackage", typ: r("ZarfPackage") },
        { json: "zarfState", js: "zarfState", typ: r("ZarfState") },
    ], false),
    "ClusterSummary": o([
        { json: "distro", js: "distro", typ: "" },
        { json: "hasZarf", js: "hasZarf", typ: true },
        { json: "reachable", js: "reachable", typ: true },
    ], false),
    "ConnectString": o([
        { json: "Description", js: "Description", typ: "" },
        { json: "Url", js: "Url", typ: "" },
    ], false),
    "ZarfCommonOptions": o([
        { json: "Confirm", js: "Confirm", typ: true },
        { json: "SetVariables", js: "SetVariables", typ: m("") },
        { json: "TempDirectory", js: "TempDirectory", typ: "" },
    ], false),
    "ZarfCreateOptions": o([
        { json: "ImageCachePath", js: "ImageCachePath", typ: "" },
        { json: "Insecure", js: "Insecure", typ: true },
        { json: "OutputDirectory", js: "OutputDirectory", typ: "" },
        { json: "SkipSBOM", js: "SkipSBOM", typ: true },
    ], false),
    "ZarfDeployOptions": o([
        { json: "ApplianceMode", js: "ApplianceMode", typ: true },
        { json: "Components", js: "Components", typ: "" },
        { json: "NodePort", js: "NodePort", typ: "" },
        { json: "PackagePath", js: "PackagePath", typ: "" },
        { json: "Secret", js: "Secret", typ: "" },
        { json: "SGetKeyPath", js: "SGetKeyPath", typ: "" },
        { json: "StorageClass", js: "StorageClass", typ: "" },
    ], false),
    "ZarfPackage": o([
        { json: "build", js: "build", typ: u(undefined, r("ZarfBuildData")) },
        { json: "components", js: "components", typ: a(r("ZarfComponent")) },
        { json: "constants", js: "constants", typ: u(undefined, a(r("ZarfPackageConstant"))) },
        { json: "kind", js: "kind", typ: r("Kind") },
        { json: "metadata", js: "metadata", typ: u(undefined, r("ZarfMetadata")) },
        { json: "seed", js: "seed", typ: u(undefined, "") },
        { json: "variables", js: "variables", typ: u(undefined, a(r("ZarfPackageVariable"))) },
    ], false),
    "ZarfBuildData": o([
        { json: "architecture", js: "architecture", typ: "" },
        { json: "terminal", js: "terminal", typ: "" },
        { json: "timestamp", js: "timestamp", typ: "" },
        { json: "user", js: "user", typ: "" },
        { json: "version", js: "version", typ: "" },
    ], false),
    "ZarfComponent": o([
        { json: "charts", js: "charts", typ: u(undefined, a(r("ZarfChart"))) },
        { json: "cosignKeyPath", js: "cosignKeyPath", typ: u(undefined, "") },
        { json: "dataInjections", js: "dataInjections", typ: u(undefined, a(r("ZarfDataInjection"))) },
        { json: "default", js: "default", typ: u(undefined, true) },
        { json: "description", js: "description", typ: u(undefined, "") },
        { json: "files", js: "files", typ: u(undefined, a(r("ZarfFile"))) },
        { json: "group", js: "group", typ: u(undefined, "") },
        { json: "images", js: "images", typ: u(undefined, a("")) },
        { json: "import", js: "import", typ: u(undefined, r("ZarfComponentImport")) },
        { json: "manifests", js: "manifests", typ: u(undefined, a(r("ZarfManifest"))) },
        { json: "name", js: "name", typ: "" },
        { json: "only", js: "only", typ: u(undefined, r("ZarfComponentOnlyTarget")) },
        { json: "repos", js: "repos", typ: u(undefined, a("")) },
        { json: "required", js: "required", typ: u(undefined, true) },
        { json: "scripts", js: "scripts", typ: u(undefined, r("ZarfComponentScripts")) },
    ], false),
    "ZarfChart": o([
        { json: "gitPath", js: "gitPath", typ: u(undefined, "") },
        { json: "name", js: "name", typ: "" },
        { json: "namespace", js: "namespace", typ: "" },
        { json: "releaseName", js: "releaseName", typ: u(undefined, "") },
        { json: "url", js: "url", typ: "" },
        { json: "valuesFiles", js: "valuesFiles", typ: u(undefined, a("")) },
        { json: "version", js: "version", typ: "" },
    ], false),
    "ZarfDataInjection": o([
        { json: "compress", js: "compress", typ: u(undefined, true) },
        { json: "source", js: "source", typ: "" },
        { json: "target", js: "target", typ: r("ZarfContainerTarget") },
    ], false),
    "ZarfContainerTarget": o([
        { json: "container", js: "container", typ: "" },
        { json: "namespace", js: "namespace", typ: "" },
        { json: "path", js: "path", typ: "" },
        { json: "selector", js: "selector", typ: "" },
    ], false),
    "ZarfFile": o([
        { json: "executable", js: "executable", typ: u(undefined, true) },
        { json: "shasum", js: "shasum", typ: u(undefined, "") },
        { json: "source", js: "source", typ: "" },
        { json: "symlinks", js: "symlinks", typ: u(undefined, a("")) },
        { json: "target", js: "target", typ: "" },
    ], false),
    "ZarfComponentImport": o([
        { json: "name", js: "name", typ: u(undefined, "") },
        { json: "path", js: "path", typ: "" },
    ], false),
    "ZarfManifest": o([
        { json: "files", js: "files", typ: u(undefined, a("")) },
        { json: "kustomizations", js: "kustomizations", typ: u(undefined, a("")) },
        { json: "kustomizeAllowAnyDirectory", js: "kustomizeAllowAnyDirectory", typ: u(undefined, true) },
        { json: "name", js: "name", typ: "" },
        { json: "namespace", js: "namespace", typ: u(undefined, "") },
    ], false),
    "ZarfComponentOnlyTarget": o([
        { json: "cluster", js: "cluster", typ: u(undefined, r("ZarfComponentOnlyCluster")) },
        { json: "localOS", js: "localOS", typ: u(undefined, r("LocalOS")) },
    ], false),
    "ZarfComponentOnlyCluster": o([
        { json: "architecture", js: "architecture", typ: u(undefined, r("Architecture")) },
        { json: "distros", js: "distros", typ: u(undefined, a("")) },
    ], false),
    "ZarfComponentScripts": o([
        { json: "after", js: "after", typ: u(undefined, a("")) },
        { json: "before", js: "before", typ: u(undefined, a("")) },
        { json: "prepare", js: "prepare", typ: u(undefined, a("")) },
        { json: "retry", js: "retry", typ: u(undefined, true) },
        { json: "showOutput", js: "showOutput", typ: u(undefined, true) },
        { json: "timeoutSeconds", js: "timeoutSeconds", typ: u(undefined, 0) },
    ], false),
    "ZarfPackageConstant": o([
        { json: "name", js: "name", typ: "" },
        { json: "value", js: "value", typ: "" },
    ], false),
    "ZarfMetadata": o([
        { json: "architecture", js: "architecture", typ: u(undefined, "") },
        { json: "description", js: "description", typ: u(undefined, "") },
        { json: "image", js: "image", typ: u(undefined, "") },
        { json: "name", js: "name", typ: "" },
        { json: "uncompressed", js: "uncompressed", typ: u(undefined, true) },
        { json: "url", js: "url", typ: u(undefined, "") },
        { json: "version", js: "version", typ: u(undefined, "") },
    ], false),
    "ZarfPackageVariable": o([
        { json: "default", js: "default", typ: u(undefined, "") },
        { json: "name", js: "name", typ: "" },
        { json: "prompt", js: "prompt", typ: u(undefined, true) },
    ], false),
    "ZarfState": o([
        { json: "agentTLS", js: "agentTLS", typ: r("GeneratedPKI") },
        { json: "architecture", js: "architecture", typ: "" },
        { json: "distro", js: "distro", typ: "" },
        { json: "nodePort", js: "nodePort", typ: "" },
        { json: "secret", js: "secret", typ: "" },
        { json: "storageClass", js: "storageClass", typ: "" },
        { json: "zarfAppliance", js: "zarfAppliance", typ: true },
    ], false),
    "GeneratedPKI": o([
        { json: "CA", js: "CA", typ: "" },
        { json: "Cert", js: "Cert", typ: "" },
        { json: "Key", js: "Key", typ: "" },
    ], false),
    "Architecture": [
        "amd64",
        "arm64",
    ],
    "LocalOS": [
        "darwin",
        "linux",
        "windows",
    ],
    "Kind": [
        "ZarfInitConfig",
        "ZarfPackageConfig",
    ],
};
