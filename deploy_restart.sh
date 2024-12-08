#!/bin/bash

# サーバーのリスト
servers=(
    "isucon@57.181.94.247" # 1台目
    # "isucon@57.182.106.98" # 2台目
    # "isucon@35.79.193.73" # 3台目
)

# デプロイ対象のディレクトリ
local_dirs=(
    "webapp/go/"
    "webapp/sql/"
)
remote_base_dir="/home/isucon/webapp"

# 再起動対象のサービス
service_name="isuride-go.service"
binary_name="isuride"  # バイナリの名前

# デプロイ処理
for server in "${servers[@]}"; do
    echo "==== Deploying to ${server} ===="

    # 各ディレクトリをコピー
    for local_dir in "${local_dirs[@]}"; do
        remote_dir="${remote_base_dir}/$(basename "${local_dir}")"
        echo "-> Copying ${local_dir} to ${server}:webapp..."
        scp -r "${local_dir}" "${server}:webapp"
        if [ $? -ne 0 ]; then
            echo "!! Failed to copy ${local_dir} to ${server}:webapp"
            exit 1
        fi
    done

    echo "Files successfully copied to ${server}"

    # リモートでのGoビルドとサービス再起動
    echo "-> Building and restarting service on ${server}..."
    ssh "${server}" bash <<EOF
        set -e  # エラー時に即終了

        echo "[Remote] Building Go application in ${remote_base_dir}/go..."
        cd "${remote_base_dir}/go"
        /home/isucon/local/golang/bin/go build -o "${binary_name}"

        echo "[Remote] Restarting service: ${service_name}..."
        sudo systemctl restart "${service_name}"
        echo "[Remote] Service ${service_name} restarted successfully"
EOF
    if [ $? -ne 0 ]; then
        echo "!! Failed to build or restart service on ${server}"
        exit 1
    fi

    echo "==== Deployment to ${server} completed successfully ===="
done

echo "==== All servers updated successfully ===="
