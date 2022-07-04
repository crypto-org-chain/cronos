// jsonnet yaml_doc.jsonnet -m . -S
{
   'low_block_gas_limit.yaml': std.manifestYamlDoc(import './low_block_gas_limit.jsonnet', true, false),
   'genesis_token_mapping.yaml': std.manifestYamlDoc(import './genesis_token_mapping.jsonnet', true, false),
   'disable_auto_deployment.yaml': std.manifestYamlDoc(import './disable_auto_deployment.jsonnet', true, false),
   'long_timeout_commit.yaml': std.manifestYamlDoc(import './long_timeout_commit.jsonnet', true, false),
   'cosmovisor.yaml': std.manifestYamlDoc(import './cosmovisor.jsonnet', true, false),
   'pruned-node.yaml': std.manifestYamlDoc(import './pruned-node.jsonnet', true, false),
}
