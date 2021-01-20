# Overview
jsonc(Json Compare) is a tool used to process json message based on a template.

No matter what kind of language you use, to check a complex json message, and even process it's content like extract field we're interested, is difficult and require a long, obscure for/if/else statements.

To avoid this, gmeter provides a template constructed from a real json message. With embedded commands and environment variables, the processing is really clear and easy to compose or modify.

# Structure
Here we gives an example that assembles all elements jsonc supports to show you a big picture:
```json
{
  "`default`": [ // "`default`" defines action for each member of this object, here it prints its key
    "`print found key $<key>`"
  ],

  "a: optional": 1, // "a" is optional, but if it is present, it must be 1
  "b": "`strlen $ | assert $(OUTPUT) > 10`", // "b" should be a string whose length is greater than 10
  "c": false, // "c" must be present and must be false

  "d": [
    // here gives special processing as first object item in list
    {
      // `list` operate on this list, and here makes sure its length > 4
      "`list`": [ "`assert $<length> > 4`" ], 

      // every item in this list must be positive number
      "`item`": [ "`assert $ > 0`" ], 

      // for rest of items in the list after explicitly defined,
      // it must greater than 10
      "`default`": [ "`assert $ > 10`" ] 
    },
    // now the content definition, here it requires the first element must be 1, 2, 3
    1,
    2,
    3
  ],

  "e": [
    {
      // `template` defines a sub-jsonc-template to check every item in this list
      "`template`": {
        "name": "strlen $<key> | assert $(OUTPUT) > 3", // all item should has a name whose length > 3
        "qty": "`assert $ > 10`" // all qty field must be a number > 10
      }
    },
    {
      // index indicates if apple must be present and its qty must be > 1000
      "name: index": "apple",  
      "qty": "`assert $ > 1000`"
    },
    {
      // index and optional indicates if pear is present, it must be < 500
      "name: index, optional": "pear",  
      "qty": "`assert $ < 500`"
    }
  ]
}
```

This is a valid json for this template:
```json
  {
    "b": "abcdefg hijklmn opq",
    "c": false,

    "d": [
      1,
      2,
      3,
      12,
      13
    ],

    "e": [
      {
        "name": "apple",
        "qty": 1200
      }
    ]
  }

```

this is an invalid json:
```json
  {
    "b": "abcdefg hijklmn opq",
    "c": false,

    "d": [
      1,
      2,
      3,
      4, // `default` require it > 10
      13
    ],

    "e": [
      {
        "name": "apple",
        "qty": 1200
      }
    ]
  }

```

# Grammar

In a template, values are defined as static or dynamic representation. Static definition indicates it defines a static value as normal json grammar like:
```json
{
    "a": 1,
    "b": "abc",
    "c": false,
    "e": [
        1, 2, 3
    ]
}
```

While static value is defined, target json item must be exactly same as this value.

Dynamic definition could be:
- "`embedded command or pipeline`"
- ["`cmd1`", "cmd2", ...]
- sub template of jsonc, for example, list `template` definition could define a template of jsonc.

While dynamic value is defined, it is called over target json item.

## Json Environment
In jsonc processing, an json environment is created for each item. By using `$<name>` you may get json environment value `name`.

Unlike local environment, json environment could not be written.

Now supported variables are:
- `key`: in processing of json object member, here stores its key string. For other types like list, it's empty.
- `value`: for basic types, it stores string representation of value, specially for bool it's `"true"` or `"false"`. For json object and list, it's marshalled json string.
- `length`: for json list or string, it's length of this list or string, for other types it's `"0"`

specially, `$` is used as a shortcut for `$<value>`
## Object
### Key Options
In an object, members are defined as `"key": <value>`, jsonc allows add one or more options after `key` like:
```json
{
	"a: optional": 1,
    "b: index, optional": 3
}
```

Here are supported options:
- `optional`: this item could be absent, but if it is present, `<value>` will be called.
- `index`: it is defined for list search, we'll discuss it later in list section.
- `absent`: if this is defined, value must be `null` or absent.

### Member Compare
In jsonc template, object could define only those we care, and only those items in target json will be compared.

For example, a template is defined:
```json
{
    "a": "`assert $ > 0`"
}
```
and we compare it to:
```json
{
    "a": 1,
    "b": 1
}
```
it will succeeds and `"b"` will be ignored.

But in case you need process items not defined explicitly, jsonc allow you define a member whose key is "`default`" followed by a dynamic rule, so that all items not defined but present in target json will be called with it:
```json
{
    "`default`": [
        "`print found $<key> value: $<value>`",
        "`assert $ > 0`"
    ]
    "a": "`assert $ > 0`"
}
```
for json:
```json
{
    "a": 1,
    "b": 2,
    "c": 3
}
```
it will print:
```
found b value: 2
found c value: 3
```

## List

List items can be defined with static or dynamic values(static and dynamic mixing usage is not supported), it will be compared one by one to the target json list. For example:
```json
[
    "`assert $ > 0`", "`assert $ >= 2`"
]
```
will succeed to compare to:
```json
[
    1, 2
]
```
but will fail to compare to:
```json
[
    1, 1
]
```

If list has a length more than template defines like:
```json
[
    1, 2, 3, 4, 5
]
```
it will fail unless you define a common segments including "`default`" rule to compare to those beyond definition:
```json
[
    {
        "`default`": "`assert $ >= 4`"
    }
    1, 2, 3
]
```
Here `4, 5` will be applied to `default` rule.

There are three other list rules:
- `item`: all list item will be applied on this rule
- `list`: list itself will be applied on this rule
- `template`: define a sub jsonc template compare to all items

For example:

```json
[
    {
        "`default`": "`assert $ >= 4`",
        "`list`": "`assert $<length> > 4`",
        "`item`": "`assert $ > 0`"
    }
    1, 2, 3
]
```
Here `list` make sure this list has at least 5 items and each item should be positive number. Note that `$<length>` is applied to the whole list and it's value is the length of this list.

For `template`:
```json
[
    {
        "`template`": {
          "qty": "`assert $ > 10`"
        }
    }
]
```
defines a list that all items should has a member `qty` and its value is more than 10 like:

[
    {
      "name": "apple",  
      "qty": 1200
    },
    {
      "name": "pear",  
      "qty": 400
    }
]


We should notify that list item could be sequence insensitive, in which case `[1, 2, 3]` and `[1, 3, 2]` are both correct. In which case you may need a search function so that template item could compare to that item matches some field, for example:
```json
[
    {
      "name: index": "apple",  
      "qty": "`assert $ > 1000`"
    },
    {
      "name: index, optional": "pear",  
      "qty": "`assert $ < 500`"
    }
  ]
```
In this template we expect:
1. `apple` is present and its `qty` is more than 1000
2. `pear` is optional, but if it's present, its `qty` should not more than 500

For this template, both these json are valid:

```json
[
    {
      "name": "apple",  
      "qty": 1200
    },
    {
      "name": "pear",  
      "qty": 400
    }
]


[
    {
      "name": "pear",  
      "qty": 400
    },
    {
      "name": "apple",  
      "qty": 1200
    }
]


[
    {
      "name": "apple",  
      "qty": 1200
    }
]
```

You should know that this json is invalid because `apple` is not found:

```json
[
    {
      "name": "pear",  
      "qty": 400
    }
]
```

and this json is invalid because there is no rule to process `orange`:

```json
[
    {
      "name": "orange",  
      "qty": 120
    },
    {
      "name": "pear",  
      "qty": 400
    }
]
```
We could add a `default` or `item` or `template` to avoid this error:

```json
[
    {
        "`item`": "`json -e .qty $`"
    }
    {
      "name: index": "apple",  
      "qty": "`assert $ > 1000`"
    },
    {
      "name: index, optional": "pear",  
      "qty": "`assert $ < 500`"
    }
  ]
```
this requiers all items should has an `qty` member.

