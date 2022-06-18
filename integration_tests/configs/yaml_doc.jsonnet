// jsonnet yaml_doc.jsonnet -m . -S
{
  'low_block_gas_limit.yaml': std.manifestYamlDoc(import './low_block_gas_limit.jsonnet', true, false),
}
