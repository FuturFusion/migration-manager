# Instance filtering

Migration Manager uses [expr-lang](https://expr-lang.org/) for selecting instances to be assigned to batches.
See [Batches](batches) for more information about batch configuration.

Expr is an expression language that uses a Go-like syntax with some more human-readable operators.

```{note}
For a full list of available functions and examples for more advanced expressions, see the full language definition at https://expr-lang.org/docs/language-definition

Migration Manager has the following extended functions:
* `path_base(string)` -- returns the final part of a path tree (e.g. /path/to/foo -> foo)
* `has_tag(<category>, <tag>)` --  returns whether the `<tag>` exists in the `<category>` (Use * for all categories)
* `matches_tag(<category>, <tag>)` -- returns whether any tag contains the text `<tag>` in the <category> (use * for all categories)
```

## Common examples

| Expression                                                             | Description                                                                                                                 |
| :---                                                                   | :---                                                                                                                        |
| `location matches 'foo'`                                               | Match the instances whose `location` contains the text `foo`                                                                |
| `location matches 'foo' or location == '/path/to/bar'`                 | Match the instances whose `location` either contains `foo` or is exactly `/path/to/bar`                                     |
| `(location matches 'foo' or location == '/path/to/bar') and cpus >= 4` | Of the instances whose `location` either contains `foo` or is exactly `/path/to/bar`, match the subset with at least 4 CPUs |
| `len(disks) > 2`                                                       | Match the instances whose number of disks is greater than 2                                                                 |
| `len(filter(disks, .name matches 'mydisk')) > 2`                       | Match the instances where the number of disks whose `name` contains `mydisk` is greater than 2                              |
| `len(disks) > 0 and disks[0].supported == true`                        | Match the instances whose first disk is `supported`                                                                         |
| `len(disks) > 1 and disks[1].supported == false`                       | Match the instances that have more than one disk, where the second disk is not `supported`                                  |
| `filter(nics, .ipv4_address != '') == len(nics)`                       | Match the instances where all NICs have a non-empty `ipv4_address`                                                          |
| `hasSuffix(location, 'ubuntu') and background_import and secure_boot`  | Match the instances whose `location` ends with `ubuntu`, and who have `background_import` and `secure_boot` enabled         |
| `hasPrefix(os, 'Windows')`                                             | Match the instances whose `os` starts with `Windows`                                                                        |
| `path_base(location) == 'ubuntu2404' and source == 'esxi01'`           | Match the instances whose `location` path has the final segment `ubuntu2404`, from the source `esxi01`                      |
| `config['key'] == 'value'`                                             | Match the instances whose `config` key-value pairs contain `value` for the key `key`                                        |
| `len(split(config['tags.mycategory'], ',')) == 3`                      | Match the instances where that have 3 tags in category `mycategory`                                                         |
| `any(split(config['tags.mycategory'], ','), # == 'tag1')`              | Match the instances that have the tag `tag1` under `mycategory`                                                             |
| `has_tag('mycategory', 'tag1')`                                        | Match the instances that have the tag `tag1` under `mycategory`                                                             |
| `matches_tag('mycategory', 'mytag')`                                   | Match the instances that have any tag under `mycategory` that contain the text `mytag`                                      |
| `has_tag('*', 'tag1')`                                                 | Match the instances that have the tag `tag1` under any category                                                             |
| `matches_tag('*', 'mytag')`                                            | Match the instances that have any tag under any category that contain the text `mytag`                                      |
| `config['vmware.resource_pool'] == 'mypool'`                           | Match the instances under the resource pool `mypool`                                                                        |

```{note}
Expressions evaluate on instances after overrides have been applied.
```

```{note}
Field names of instances are the same as the `properties` field of the JSON or YAML representation shown over the API or in the `migration-manager instance show` commands with the addition of `source` and `source_type`.
```
