resource "oci_core_vcn" "test_vcn" {
    compartment_id = "my_compartment"

    cidr_blocks = ["10.0.0.0/8"]
}
