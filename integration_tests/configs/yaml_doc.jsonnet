// jsonnet yaml_doc.jsonnet -m . -S
{
   'low_block_gas_limit.yaml': std.manifestYamlDoc(import './low_block_gas_limit.jsonnet', true, false),
   'genesis_token_mapping.yaml': std.manifestYamlDoc(import './genesis_token_mapping.jsonnet', true, false),
}
