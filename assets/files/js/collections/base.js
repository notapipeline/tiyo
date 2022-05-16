class Base {
    $properties = null;

    getPropertiesForm(objectType) {
        $.get('/objectforms?object=' + objectType)
    }
}
