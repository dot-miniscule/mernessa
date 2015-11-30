//----------- TAB SELECTION ------------//

jQuery(document).ready(function() {

  jQuery('.tabs .tab-links a').on('click', function(e) {

    var currentAttrValue = jQuery(this).attr('href');

    //Show and hide tabs
    jQuery('.tabs' + currentAttrValue).fadeIn(400).siblings().hide();

    //Change and remove current tab to active
    jQuery(this).parent('li').addClass('active').siblings().removeClass('active');

    e.preventDefault(); 
  });

});
