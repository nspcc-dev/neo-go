# Conventions

This document will list conventions that this repo should follow. These are guidelines and if you believe that one should not be followed, then please state why in your PR. If you believe that a piece of code does not follow one of the conventions listed, then please open an issue before making any changes. 

When submitting a new convention, please open an issue for discussion, if possible please highlight parts in the code where this convention could help the code readiblity or simplicity.


## Avoid Named return paramters


func example(test int) (num int) {
    a = test + 1
    num = a * test
    return
}

In the above function we have used a named return paramter, which allows you to include a simple return statement without the variables you are returning. This practice can cause confusion when functions become large or the logic becomes complex, so these should be avoided.
