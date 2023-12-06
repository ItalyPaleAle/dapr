import { argv, env, exit } from 'node:process'
import { writeFileSync } from 'node:fs'

/**
 * List of all components
 * @type {Record<string,ComponentTestProperties>}
 */
const components = {
    'actorstore.postgresql': {
        conformance: true,
        conformanceSetup: 'docker-compose.sh postgresql',
    },
    'actorstore.sqlite': {
        conformance: true,
    },
}

/**
 * Type for the objects in the components dictionary
 * @typedef {Object} ComponentTestProperties
 * @property {boolean?} conformance If true, enables for conformance tests
 * @property {boolean?} certification If true, enables for certification tests
 * @property {string[]?} requiredSecrets Required secrets (if not empty, test becomes "cloud-only")
 * @property {string[]?} requiredCerts Required certs (if not empty, test becomes "cloud-only")
 * @property {boolean?} requireAWSCredentials If true, requires AWS credentials and makes the test "cloud-only"
 * @property {boolean?} requireGCPCredentials If true, requires GCP credentials and makes the test "cloud-only"
 * @property {boolean?} requireCloudflareCredentials If true, requires Cloudflare credentials and makes the test "cloud-only"
 * @property {boolean?} requireTerraform If true, requires Terraform
 * @property {boolean?} requireKind If true, requires KinD
 * @property {string?} conformanceSetup Setup script for conformance tests
 * @property {string?} conformanceDestroy Destroy script for conformance tests
 * @property {string?} certificationSetup Setup script for certification tests
 * @property {string?} certificationDestroy Destroy script for certification tests
 * @property {string?} nodeJsVersion If set, installs the specified Node.js version
 * @property {string?} mongoDbVersion If set, installs the specified MongoDB version
 * @property {string|string[]?} sourcePkg If set, sets the specified source package
 */

/**
 * Test matrix object
 * @typedef {Object} TestMatrixElement
 * @property {string} component Component name
 * @property {string?} required-secrets Required secrets
 * @property {string?} required-certs Required certs
 * @property {boolean?} require-aws-credentials Requires AWS credentials
 * @property {boolean?} require-gcp-credentials Requires GCP credentials
 * @property {boolean?} require-cloudflare-credentials Requires Cloudflare credentials
 * @property {boolean?} require-terraform Requires Terraform
 * @property {boolean?} require-kind Requires KinD
 * @property {string?} setup-script Setup script
 * @property {string?} destroy-script Destroy script
 * @property {string?} nodejs-version Install the specified Node.js version if set
 * @property {string?} mongodb-version Install the specified MongoDB version if set
 * @property {string?} source-pkg Source package
 */

/**
 * Returns the list of components for the matrix.
 * @param {'conformance'|'certification'} testKind Kind of test
 * @param {boolean} enableCloudTests If true, returns components that require secrets or credentials too (which can't be used as part of the regular CI in a PR)
 * @returns {TestMatrixElement[]} Test matrix object
 */
function GenerateMatrix(testKind, enableCloudTests) {
    /** @type {TestMatrixElement[]} */
    const res = []
    for (const name in components) {
        const comp = components[name]
        if (!comp[testKind]) {
            continue
        }

        // Skip cloud-only tests if enableCloudTests is false
        if (!enableCloudTests) {
            if (
                comp.requiredSecrets?.length ||
                comp.requiredCerts?.length ||
                comp.requireAWSCredentials ||
                comp.requireGCPCredentials ||
                comp.requireCloudflareCredentials
            ) {
                continue
            }
        } else {
            // For conformance tests, avoid running Docker and Cloud Tests together.
            if (comp.conformance && comp.requireDocker) {
                continue
            }
        }

        if (comp.sourcePkg) {
            // Ensure it's an array
            if (!Array.isArray(comp.sourcePkg)) {
                comp.sourcePkg = [comp.sourcePkg]
            }
        } else {
            // Default is to use the component name, replacing dots with /
            comp.sourcePkg = [name.replace(/\./g, '/')]
        }

        // Add the component to the array
        res.push({
            component: name,
            'required-secrets': comp.requiredSecrets?.length
                ? comp.requiredSecrets.join(',')
                : undefined,
            'required-certs': comp.requiredCerts?.length
                ? comp.requiredCerts.join(',')
                : undefined,
            'require-aws-credentials': comp.requireAWSCredentials
                ? 'true'
                : undefined,
            'require-gcp-credentials': comp.requireGCPCredentials
                ? 'true'
                : undefined,
            'require-cloudflare-credentials': comp.requireCloudflareCredentials
                ? 'true'
                : undefined,
            'require-terraform': comp.requireTerraform ? 'true' : undefined,
            'require-kind': comp.requireKind ? 'true' : undefined,
            'setup-script': comp[testKind + 'Setup'] || undefined,
            'destroy-script': comp[testKind + 'Destroy'] || undefined,
            'nodejs-version': comp.nodeJsVersion || undefined,
            'mongodb-version': comp.mongoDbVersion || undefined,
            'source-pkg': comp.sourcePkg
                .map((p) => 'github.com/dapr/components-contrib/' + p)
                .join(','),
        })
    }

    return res
}

// Upon invocation, writes the matrix to the $GITHUB_OUTPUT file
if (!env.GITHUB_OUTPUT) {
    console.error('Missing environmental variable GITHUB_OUTPUT')
    exit(1)
}
if (argv.length < 3 || !['conformance', 'certification'].includes(argv[2])) {
    console.error("First parameter must be 'conformance' or 'certification'")
    exit(1)
}
if (argv.length < 4 || !['true', 'false'].includes(argv[3])) {
    console.error("First parameter must be 'true' or 'false'")
    exit(1)
}

const testKind = argv[2]
const enableCloudTests = argv[3] == 'true'
const matrixObj = GenerateMatrix(testKind, enableCloudTests)
console.log('Generated matrix:\n\n' + JSON.stringify(matrixObj, null, '  '))

writeFileSync(env.GITHUB_OUTPUT, 'test-matrix=' + JSON.stringify(matrixObj))
