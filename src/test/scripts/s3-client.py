#!/usr/bin/env python3.13
"""
S3 Client Helper Script using boto3

Usage:
    ./s3-client.py list                                        # List buckets
    ./s3-client.py create <bucket>                             # Create bucket
    ./s3-client.py delete <bucket>                             # Delete bucket
    ./s3-client.py head <bucket>                               # Check bucket exists
    ./s3-client.py upload <bucket> <key> <file>                # Upload object
    ./s3-client.py get <bucket> <key> [file]                   # Get object (saves to file or stdout)
    ./s3-client.py head-object <bucket> <key>                  # Get object metadata without downloading
    ./s3-client.py delete-object <bucket> <key>                # Delete an object from bucket
    ./s3-client.py list-objects <bucket> [prefix] [delimiter]  # List objects in bucket (with optional prefix/delimiter)
    ./s3-client.py share <bucket> <key> [expires]              # Create a public share link (expires in seconds, 0=no expiration)
    ./s3-client.py download-shared <token> [file]              # Download object using share link token
    ./s3-client.py delete-share <token>                        # Delete a share link


Environment variables:
    S3_ACCESS_KEY_ID     - Access key (default: testkey)
    S3_SECRET_ACCESS_KEY - Secret key (default: testsecret)
    S3_ENDPOINT          - Endpoint URL (default: http://localhost:8089)
    S3_REGION            - AWS region (default: us-east-1)
"""

import os
import sys
import boto3
import requests
from botocore.exceptions import ClientError
from botocore.auth import SigV4Auth
from botocore.awsrequest import AWSRequest

# Configuration from environment variables
ACCESS_KEY = os.environ.get('S3_ACCESS_KEY_ID', 'testkey')
SECRET_KEY = os.environ.get('S3_SECRET_ACCESS_KEY', 'testsecret')
ENDPOINT = os.environ.get('S3_ENDPOINT', 'http://localhost:8089')
REGION = os.environ.get('S3_REGION', 'us-east-1')


def get_client():
    """Create and return an S3 client."""
    return boto3.client(
        's3',
        endpoint_url=ENDPOINT,
        aws_access_key_id=ACCESS_KEY,
        aws_secret_access_key=SECRET_KEY,
        region_name=REGION,
    )


def list_buckets():
    """List all buckets."""
    client = get_client()
    response = client.list_buckets()

    print(f"Owner: {response.get('Owner', {}).get('DisplayName', 'N/A')}")
    print("\nBuckets:")

    buckets = response.get('Buckets', [])
    if not buckets:
        print("  (none)")
    else:
        for bucket in buckets:
            print(f"  - {bucket['Name']} (created: {bucket['CreationDate']})")

    return response


def create_bucket(bucket_name):
    """Create a bucket."""
    client = get_client()

    try:
        response = client.create_bucket(Bucket=bucket_name)
        print(f"Bucket '{bucket_name}' created successfully")
        print(f"Location: {response.get('Location', 'N/A')}")
        return response
    except ClientError as e:
        print(f"Error creating bucket: {e.response['Error']['Code']} - {e.response['Error']['Message']}")
        sys.exit(1)


def delete_bucket(bucket_name):
    """Delete a bucket."""
    client = get_client()

    try:
        client.delete_bucket(Bucket=bucket_name)
        print(f"Bucket '{bucket_name}' deleted successfully")
    except ClientError as e:
        print(f"Error deleting bucket: {e.response['Error']['Code']} - {e.response['Error']['Message']}")
        sys.exit(1)


def head_bucket(bucket_name):
    """Check if a bucket exists."""
    client = get_client()

    try:
        client.head_bucket(Bucket=bucket_name)
        print(f"Bucket '{bucket_name}' exists")
    except ClientError as e:
        error_code = e.response['Error']['Code']
        if error_code == '404':
            print(f"Bucket '{bucket_name}' does not exist")
        else:
            print(f"Error checking bucket: {error_code} - {e.response['Error']['Message']}")
        sys.exit(1)


def upload_object(bucket_name, key, file_path):
    """Upload an object to a bucket."""
    client = get_client()

    if not os.path.exists(file_path):
        print(f"Error: file '{file_path}' not found")
        sys.exit(1)

    file_size = os.path.getsize(file_path)

    try:
        with open(file_path, 'rb') as f:
            response = client.put_object(
                Bucket=bucket_name,
                Key=key,
                Body=f,
            )
        print(f"Object '{key}' uploaded successfully to bucket '{bucket_name}'")
        print(f"  File size: {file_size} bytes")
        print(f"  ETag: {response.get('ETag', 'N/A')}")
        return response
    except ClientError as e:
        print(f"Error uploading object: {e.response['Error']['Code']} - {e.response['Error']['Message']}")
        sys.exit(1)


def get_object(bucket_name, key, output_file=None):
    """Get an object from a bucket."""
    client = get_client()

    try:
        response = client.get_object(Bucket=bucket_name, Key=key)

        body = response['Body'].read()

        if output_file:
            with open(output_file, 'wb') as f:
                f.write(body)
            print(f"Object '{key}' saved to '{output_file}'")
        else:
            # Write to stdout
            sys.stdout.buffer.write(body)
            sys.stdout.buffer.flush()
            return response

        print(f"  Size: {response.get('ContentLength', 'N/A')} bytes")
        print(f"  ETag: {response.get('ETag', 'N/A')}")
        print(f"  Content-Type: {response.get('ContentType', 'N/A')}")
        print(f"  Last-Modified: {response.get('LastModified', 'N/A')}")
        return response
    except ClientError as e:
        print(f"Error getting object: {e.response['Error']['Code']} - {e.response['Error']['Message']}")
        sys.exit(1)


def head_object(bucket_name, key):
    """Get object metadata without downloading the object."""
    client = get_client()

    try:
        response = client.head_object(Bucket=bucket_name, Key=key)
        print(f"Object '{key}' metadata from bucket '{bucket_name}':")
        print(f"  Content-Length: {response.get('ContentLength', 'N/A')} bytes")
        print(f"  ETag: {response.get('ETag', 'N/A')}")
        print(f"  Content-Type: {response.get('ContentType', 'N/A')}")
        print(f"  Last-Modified: {response.get('LastModified', 'N/A')}")
        return response
    except ClientError as e:
        print(f"Error getting object metadata: {e.response['Error']['Code']} - {e.response['Error']['Message']}")
        sys.exit(1)


def delete_object(bucket_name, key):
    """Delete an object from a bucket."""
    client = get_client()

    try:
        client.delete_object(Bucket=bucket_name, Key=key)
        print(f"Object '{key}' deleted successfully from bucket '{bucket_name}'")
    except ClientError as e:
        print(f"Error deleting object: {e.response['Error']['Code']} - {e.response['Error']['Message']}")
        sys.exit(1)


def list_objects(bucket_name, prefix=None, delimiter=None):
    """List objects in a bucket with optional prefix and delimiter filtering."""
    client = get_client()

    try:
        # Build request parameters
        params = {'Bucket': bucket_name}
        if prefix:
            params['Prefix'] = prefix
        if delimiter:
            params['Delimiter'] = delimiter

        response = client.list_objects_v2(**params)

        print(f"Objects in bucket '{bucket_name}':")
        if prefix:
            print(f"  Prefix: {prefix}")
        if delimiter:
            print(f"  Delimiter: {delimiter}")

        # Show common prefixes (virtual folders) if using delimiter
        common_prefixes = response.get('CommonPrefixes', [])
        if common_prefixes:
            print("\n  Folders:")
            for cp in common_prefixes:
                print(f"    📁 {cp['Prefix']}")

        contents = response.get('Contents', [])
        if not contents and not common_prefixes:
            print("  (none)")
        elif contents:
            print("\n  Objects:")
            for obj in contents:
                print(f"    - {obj['Key']} (size: {obj['Size']}, modified: {obj['LastModified']})")

        print(f"\n  Total objects: {response.get('KeyCount', len(contents))}")
        if response.get('IsTruncated'):
            print("  (results truncated)")

        return response
    except ClientError as e:
        print(f"Error listing objects: {e.response['Error']['Code']} - {e.response['Error']['Message']}")
        sys.exit(1)


def create_share_link(bucket_name, key, expires_in=0):
    """Create a public share link for an object."""
    # Parse endpoint URL
    endpoint_base = ENDPOINT.rstrip('/')
    url = f"{endpoint_base}/share/create/{bucket_name}/{key}?expires_in={expires_in}"

    # Create AWS credentials
    client = get_client()
    credentials = client._request_signer._credentials

    # Create and sign the request
    request = AWSRequest(method='POST', url=url)
    SigV4Auth(credentials, 's3', REGION).add_auth(request)

    # Make the request
    try:
        response = requests.post(
            url,
            headers=dict(request.headers)
        )
        response.raise_for_status()

        data = response.json()
        print(f"Share link created for '{bucket_name}/{key}':")
        print(f"  Token: {data['token']}")
        print(f"  Share URL: {data['share_url']}")
        if expires_in > 0:
            print(f"  Expires in: {expires_in} seconds")
        else:
            print(f"  Expires: Never")

        return data
    except requests.exceptions.RequestException as e:
        print(f"Error creating share link: {e}")
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_data = e.response.json()
                print(f"  {error_data.get('Code', 'Error')}: {error_data.get('Message', str(e))}")
            except:
                print(f"  Response: {e.response.text}")
        sys.exit(1)


def download_shared_object(token, output_file=None):
    """Download an object using a share link token."""
    endpoint_base = ENDPOINT.rstrip('/')
    url = f"{endpoint_base}/share/{token}"

    try:
        response = requests.get(url, stream=True)
        response.raise_for_status()

        if output_file:
            with open(output_file, 'wb') as f:
                for chunk in response.iter_content(chunk_size=8192):
                    f.write(chunk)
            print(f"Shared object downloaded to '{output_file}'")
            print(f"  Size: {response.headers.get('Content-Length', 'N/A')} bytes")
            print(f"  ETag: {response.headers.get('ETag', 'N/A')}")
            print(f"  Content-Type: {response.headers.get('Content-Type', 'N/A')}")
            print(f"  Last-Modified: {response.headers.get('Last-Modified', 'N/A')}")
        else:
            # Write to stdout
            sys.stdout.buffer.write(response.content)
            sys.stdout.buffer.flush()

        return response
    except requests.exceptions.RequestException as e:
        print(f"Error downloading shared object: {e}")
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_data = e.response.json()
                print(f"  {error_data.get('Code', 'Error')}: {error_data.get('Message', str(e))}")
            except:
                print(f"  Response: {e.response.text}")
        sys.exit(1)


def delete_share_link(token):
    """Delete a share link."""
    endpoint_base = ENDPOINT.rstrip('/')
    url = f"{endpoint_base}/share/{token}"

    # Create AWS credentials
    client = get_client()
    credentials = client._request_signer._credentials

    # Create and sign the request
    request = AWSRequest(method='DELETE', url=url)
    SigV4Auth(credentials, 's3', REGION).add_auth(request)

    try:
        response = requests.delete(
            url,
            headers=dict(request.headers)
        )
        response.raise_for_status()

        print(f"Share link deleted successfully")
        print(f"  Token: {token}")
        return response
    except requests.exceptions.RequestException as e:
        print(f"Error deleting share link: {e}")
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_data = e.response.json()
                print(f"  {error_data.get('Code', 'Error')}: {error_data.get('Message', str(e))}")
            except:
                print(f"  Response: {e.response.text}")
        sys.exit(1)


def print_usage():
    """Print usage information."""
    print(__doc__)
    print("Current configuration:")
    print(f"  Endpoint:   {ENDPOINT}")
    print(f"  Region:     {REGION}")
    print(f"  Access Key: {ACCESS_KEY}")


def main():
    if len(sys.argv) < 2:
        print_usage()
        sys.exit(1)

    command = sys.argv[1].lower()

    if command == 'list':
        list_buckets()
    elif command == 'create':
        if len(sys.argv) < 3:
            print("Error: bucket name required")
            print("Usage: ./s3-client.py create <bucket>")
            sys.exit(1)
        create_bucket(sys.argv[2])
    elif command == 'delete':
        if len(sys.argv) < 3:
            print("Error: bucket name required")
            print("Usage: ./s3-client.py delete <bucket>")
            sys.exit(1)
        delete_bucket(sys.argv[2])
    elif command == 'head':
        if len(sys.argv) < 3:
            print("Error: bucket name required")
            print("Usage: ./s3-client.py head <bucket>")
            sys.exit(1)
        head_bucket(sys.argv[2])
    elif command == 'upload':
        if len(sys.argv) < 5:
            print("Error: bucket, key, and file path required")
            print("Usage: ./s3-client.py upload <bucket> <key> <file>")
            sys.exit(1)
        upload_object(sys.argv[2], sys.argv[3], sys.argv[4])
    elif command == 'get':
        if len(sys.argv) < 4:
            print("Error: bucket and key required")
            print("Usage: ./s3-client.py get <bucket> <key> [file]")
            sys.exit(1)
        output_file = sys.argv[4] if len(sys.argv) > 4 else None
        get_object(sys.argv[2], sys.argv[3], output_file)
    elif command == 'head-object':
        if len(sys.argv) < 4:
            print("Error: bucket and key required")
            print("Usage: ./s3-client.py head-object <bucket> <key>")
            sys.exit(1)
        head_object(sys.argv[2], sys.argv[3])
    elif command == 'delete-object':
        if len(sys.argv) < 4:
            print("Error: bucket and key required")
            print("Usage: ./s3-client.py delete-object <bucket> <key>")
            sys.exit(1)
        delete_object(sys.argv[2], sys.argv[3])
    elif command == 'list-objects':
        if len(sys.argv) < 3:
            print("Error: bucket name required")
            print("Usage: ./s3-client.py list-objects <bucket> [prefix] [delimiter]")
            sys.exit(1)
        prefix = sys.argv[3] if len(sys.argv) > 3 else None
        delimiter = sys.argv[4] if len(sys.argv) > 4 else None
        list_objects(sys.argv[2], prefix, delimiter)
    elif command == 'share':
        if len(sys.argv) < 4:
            print("Error: bucket and key required")
            print("Usage: ./s3-client.py share <bucket> <key> [expires_in]")
            sys.exit(1)
        expires_in = int(sys.argv[4]) if len(sys.argv) > 4 else 0
        create_share_link(sys.argv[2], sys.argv[3], expires_in)
    elif command == 'download-shared':
        if len(sys.argv) < 3:
            print("Error: token required")
            print("Usage: ./s3-client.py download-shared <token> [file]")
            sys.exit(1)
        output_file = sys.argv[3] if len(sys.argv) > 3 else None
        download_shared_object(sys.argv[2], output_file)
    elif command == 'delete-share':
        if len(sys.argv) < 3:
            print("Error: token required")
            print("Usage: ./s3-client.py delete-share <token>")
            sys.exit(1)
        delete_share_link(sys.argv[2])
    elif command in ['help', '-h', '--help']:
        print_usage()
    else:
        print(f"Unknown command: {command}")
        print_usage()
        sys.exit(1)


if __name__ == '__main__':
    main()
